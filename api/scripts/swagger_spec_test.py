"""Regression tests for the complete Swagger 2 compatibility specification."""

from copy import deepcopy
from pathlib import Path
import unittest

import yaml

from swagger_spec import comparison_spec, operation, parameter, project, render, response


class SwaggerSpecTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.api_dir = Path(__file__).resolve().parents[1]
        cls.spec = yaml.safe_load((cls.api_dir / "openapi.yaml").read_text())
        cls.swagger = (cls.api_dir / "swagger.yaml").read_text()

    def test_checked_in_swagger_is_up_to_date(self):
        self.assertEqual(
            comparison_spec(project(self.spec)), comparison_spec(yaml.safe_load(self.swagger)),
            "Run make -C api swagger-spec and commit the generated swagger.yaml",
        )

    def test_unchanged_swagger_keeps_exact_layout(self):
        self.assertEqual(render(self.spec, self.swagger), self.swagger)

    def test_generation_without_existing_swagger_matches_checked_in_content(self):
        self.assertEqual(
            comparison_spec(yaml.safe_load(render(self.spec))),
            comparison_spec(yaml.safe_load(self.swagger)),
        )

    def test_source_changes_are_not_hidden_by_existing_swagger(self):
        source = deepcopy(self.spec)
        source["info"]["title"] = "Updated Engine API"
        result = render(source, self.swagger)
        self.assertNotEqual(result, self.swagger)
        self.assertEqual(yaml.safe_load(result)["info"]["title"], "Updated Engine API")

    def test_source_property_order_changes_are_not_hidden(self):
        source = deepcopy(self.spec)
        schema = source["components"]["schemas"]["MountPoint"]
        schema["properties"] = dict(reversed(list(schema["properties"].items())))
        result = render(source, self.swagger)
        self.assertNotEqual(result, self.swagger)
        self.assertEqual(
            list(yaml.safe_load(result)["definitions"]["MountPoint"]["properties"]),
            list(schema["properties"]),
        )

    def test_repeated_parameters_and_omitted_array_default(self):
        result = project(self.spec)
        for path, method, name in (
            ("/images/create", "post", "changes"),
            ("/images/get", "get", "names"),
            ("/images/{name}", "delete", "platforms"),
        ):
            with self.subTest(path=path):
                param = next(p for p in result["paths"][path][method]["parameters"] if p["name"] == name)
                self.assertEqual(param["collectionFormat"], "multi")
        managers = result["definitions"]["SwarmInfo"]["properties"]["RemoteManagers"]
        self.assertNotIn("default", managers)

    def test_complete_api_and_schema_order(self):
        original = deepcopy(self.spec)
        result = project(self.spec)
        self.assertEqual(result["swagger"], "2.0")
        self.assertEqual(result["basePath"], "/v1.56")
        self.assertEqual(result["info"]["version"], "1.56")
        self.assertEqual(set(result["paths"]), set(self.spec["paths"]))
        for path, methods in self.spec["paths"].items():
            self.assertEqual(set(result["paths"][path]), set(methods))
        self.assertEqual(list(result["definitions"]), list(self.spec["components"]["schemas"]))
        self.assertEqual(self.spec, original)

    def test_current_api_additions_survive_conversion(self):
        result = project(self.spec)
        attestations = result["paths"]["/images/{name}/attestations"]["get"]
        self.assertEqual(attestations["responses"]["200"]["schema"], {
            "type": "array", "items": {"$ref": "#/definitions/AttestationStatement"},
        })
        self.assertEqual(
            result["paths"]["/containers/{id}/start"]["post"]["responses"]["400"]["schema"],
            {"$ref": "#/definitions/ErrorResponse"},
        )
        self.assertIn("Umask", result["definitions"]["HostConfig"]["allOf"][1]["properties"])
        self.assertEqual(
            result["definitions"]["Task"]["properties"]["NetworksAttachments"]["items"],
            {"$ref": "#/definitions/NetworkAttachment"},
        )

    def test_repeated_query_keys(self):
        for explode, expected in ((True, "multi"), (False, "csv")):
            with self.subTest(explode=explode):
                self.assertEqual(parameter({
                    "name": "names", "in": "query", "style": "form", "explode": explode,
                    "schema": {"type": "array", "items": {"type": "string"}},
                }), {
                    "name": "names", "in": "query", "type": "array",
                    "items": {"type": "string"}, "collectionFormat": expected,
                })

    def test_request_body_and_response_media_types(self):
        result = operation({
            "requestBody": {
                "x-codegen-request-body-name": "archive", "required": True,
                "description": "Archive to import",
                "content": {"application/x-tar": {"schema": {"type": "string", "format": "binary"}}},
            },
            "responses": {
                "200": {"description": "stream", "content": {"application/octet-stream": {}}},
                "400": {"description": "bad request", "content": {"application/json": {
                    "schema": {"$ref": "#/components/schemas/ErrorResponse"},
                    "example": {"message": "bad archive"},
                }}},
            },
        }, {"consumes": ["application/json"], "produces": ["application/json"]})
        self.assertEqual(result["consumes"], ["application/x-tar"])
        self.assertEqual(result["parameters"], [{
            "name": "archive", "in": "body", "required": True,
            "description": "Archive to import", "schema": {"type": "string", "format": "binary"},
        }])
        self.assertEqual(result["produces"], ["application/octet-stream", "application/json"])
        self.assertEqual(result["responses"]["200"], {"description": "stream"})
        self.assertEqual(result["responses"]["400"], {
            "description": "bad request", "schema": {"$ref": "#/definitions/ErrorResponse"},
            "examples": {"application/json": {"message": "bad archive"}},
        })

    def test_head_responses_have_headers_but_no_bodies(self):
        result = project(self.spec)
        for path in ("/_ping", "/containers/{id}/archive"):
            head = result["paths"][path]["head"]
            self.assertEqual(head["produces"], [])
            self.assertTrue(head["responses"]["200"]["headers"])
            for item in head["responses"].values():
                self.assertNotIn("schema", item)
                self.assertNotIn("examples", item)

    def test_json_errors_contribute_to_operation_media_types(self):
        paths = project(self.spec)["paths"]
        self.assertEqual(paths["/_ping"]["get"]["produces"], ["text/plain", "application/json"])
        self.assertEqual(paths["/containers/{id}/archive"]["get"]["produces"], ["application/x-tar", "application/json"])
        self.assertEqual(paths["/containers/{id}/start"]["post"]["produces"], ["application/json"])
        self.assertEqual(paths["/containers/{id}/resize"]["post"]["produces"], ["application/json"])

    def test_unrepresentable_content_fails(self):
        for content in (
            {"application/json": {"schema": {"type": "object"}}, "text/plain": {"schema": {"type": "string"}}},
            {"application/json": {"schema": {"type": "object"}}, "text/plain": {}},
            {"application/json": {"itemSchema": {"type": "object"}}},
        ):
            with self.subTest(content=content), self.assertRaises(ValueError):
                response({"description": "response", "content": content})

    def test_unsupported_operation_fields_fail(self):
        with self.assertRaises(ValueError):
            operation({"callbacks": {}, "responses": {}}, {"consumes": [], "produces": []})


if __name__ == "__main__":
    unittest.main()
