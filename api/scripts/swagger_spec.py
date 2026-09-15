"""Generate the Engine's Swagger 2.0 compatibility specification from OpenAPI.

Swagger 2 has operation-wide media types, not per-response media types. The
compatibility document uses their union; openapi.yaml remains authoritative.
"""

import sys
from copy import deepcopy
from pathlib import Path

import yaml

from swagger_model_spec import project as project_models, project_schema


def check_keys(value, allowed):
    unsupported = value.keys() - set(allowed)
    unsupported = {key for key in unsupported if not key.startswith("x-")}
    if unsupported:
        raise ValueError(f"unsupported OpenAPI fields: {sorted(unsupported)}")


def extensions(value):
    return {key: item for key, item in value.items() if key.startswith("x-")}


def parameter(value):
    check_keys(value, ("name", "in", "description", "required", "schema", "style", "explode"))
    result = {key: item for key, item in value.items() if key not in ("schema", "style", "explode")}
    schema = project_schema(value["schema"])
    if schema.get("type") not in ("string", "integer", "number", "boolean", "array"):
        raise ValueError("Swagger non-body parameters require a primitive or array type")
    result.update(schema)
    if schema["type"] == "array":
        if value["in"] != "query" or value.get("style", "form") != "form":
            raise ValueError("only form-style query arrays are supported")
        result["collectionFormat"] = "multi" if value.get("explode", True) else "csv"
    elif "style" in value or "explode" in value:
        raise ValueError("serialization options are only supported on query arrays")
    return result


def content_schema(content):
    schemas = []
    for media in content.values():
        check_keys(media, ("schema", "example"))
        schemas.append(media.get("schema"))
    if not schemas or any(schema != schemas[0] for schema in schemas):
        raise ValueError("Swagger requires the same schema for all media types of a body")
    if schemas[0] is None:
        return None
    return project_schema(schemas[0])


def response(value):
    check_keys(value, ("description", "headers", "content"))
    result = {"description": value["description"], **extensions(value)}
    if "headers" in value:
        result["headers"] = {}
        for name, header in value["headers"].items():
            check_keys(header, ("description", "schema"))
            result["headers"][name] = {
                **{key: item for key, item in header.items() if key != "schema"},
                **project_schema(header["schema"]),
            }
    if "content" in value:
        schema = content_schema(value["content"])
        if schema is not None:
            result["schema"] = schema
        examples = {
            media_type: media["example"]
            for media_type, media in value["content"].items() if "example" in media
        }
        if examples:
            result["examples"] = examples
    return result


def operation(value, defaults):
    check_keys(value, ("summary", "description", "operationId", "tags", "parameters", "requestBody", "responses", "deprecated"))
    result = {key: item for key, item in value.items() if key not in ("requestBody", "responses", "parameters", "x-consumes")}
    if "parameters" in value:
        result["parameters"] = [parameter(item) for item in value["parameters"]]
    consumes = value.get("x-consumes", defaults["consumes"])
    if "requestBody" in value:
        body = value["requestBody"]
        check_keys(body, ("description", "required", "content"))
        if "x-consumes" in value:
            raise ValueError("requestBody and x-consumes cannot both specify media types")
        consumes = list(body["content"])
        schema = content_schema(body["content"])
        if schema is None:
            raise ValueError("request bodies require a schema")
        body_parameter = {
            "name": body["x-codegen-request-body-name"], "in": "body",
            **{key: item for key, item in body.items() if key in ("description", "required")},
            "schema": schema,
        }
        result.setdefault("parameters", []).append(body_parameter)
    if consumes != defaults["consumes"]:
        result["consumes"] = consumes
    produces = list(dict.fromkeys(
        media_type for item in value["responses"].values()
        for media_type in item.get("content", {})
    ))
    if produces != defaults["produces"]:
        result["produces"] = produces
    result["responses"] = {str(code): response(item) for code, item in value["responses"].items()}
    return result


def project(spec):
    check_keys(spec, ("openapi", "jsonSchemaDialect", "servers", "info", "tags", "components", "paths"))
    if spec.get("openapi") != "3.2.0":
        raise ValueError("Swagger projection requires OpenAPI 3.2.0")
    if spec.get("jsonSchemaDialect") != "https://spec.openapis.org/oas/3.1/dialect/base":
        raise ValueError("unsupported schema dialect")
    if len(spec["servers"]) != 1 or set(spec["servers"][0]) != {"url"}:
        raise ValueError("Swagger projection requires one relative server URL")
    base_path = spec["servers"][0]["url"]
    if not base_path.startswith("/") or base_path.startswith("//") or "{" in base_path:
        raise ValueError("Swagger projection requires a relative server URL without variables")
    check_keys(spec["components"], ("schemas",))
    result = {
        "swagger": "2.0",
        "schemes": spec["x-schemes"],
        "produces": spec["x-produces"],
        "consumes": spec["x-consumes"],
        "basePath": base_path,
        "info": spec["info"],
        "tags": spec["tags"],
        "definitions": project_models(spec)["definitions"],
        "paths": {},
    }
    for path, item in spec["paths"].items():
        check_keys(item, ("get", "put", "post", "delete", "options", "head", "patch"))
        result["paths"][path] = {
            method: value if method.startswith("x-") else operation(value, result)
            for method, value in item.items()
        }
    return result


class Dumper(yaml.SafeDumper):
    def ignore_aliases(self, data):
        return True


def represent_string(dumper, value):
    return dumper.represent_scalar("tag:yaml.org,2002:str", value, style="|" if "\n" in value else None)


Dumper.add_representer(str, represent_string)


def comparison_spec(spec):
    """Ignore parameter placement and YAML status-key types, but not schema order."""
    spec = deepcopy(spec)

    def schema_order(schema):
        if isinstance(schema, bool):
            return
        if "properties" in schema:
            for item in schema["properties"].values():
                schema_order(item)
            schema["properties"] = list(schema["properties"].items())
        for key in ("items", "additionalProperties"):
            if key in schema:
                schema_order(schema[key])
        for item in schema.get("allOf", []):
            schema_order(item)

    for schema in spec["definitions"].values():
        schema_order(schema)
    for methods in spec["paths"].values():
        for method, op in methods.items():
            if method.startswith("x-"):
                continue
            op["responses"] = {str(code): value for code, value in op["responses"].items()}
            for value in op["responses"].values():
                if "schema" in value:
                    schema_order(value["schema"])
            for value in op.get("parameters", []):
                if "schema" in value:
                    schema_order(value["schema"])
            op["parameters"] = sorted(op.get("parameters", []), key=lambda p: (p["in"], p["name"]))
    return spec


def render(spec, previous=""):
    projected = project(spec)
    if previous and comparison_spec(yaml.safe_load(previous)) == comparison_spec(projected):
        return previous
    return "# Code generated from openapi.yaml; DO NOT EDIT.\n" + yaml.dump(
        projected, Dumper=Dumper, sort_keys=False, allow_unicode=True, width=1000,
    )


if __name__ == "__main__":
    previous = ""
    if len(sys.argv) > 2 and Path(sys.argv[2]).exists():
        previous = Path(sys.argv[2]).read_text(encoding="utf-8")
    with open(sys.argv[1], encoding="utf-8") as source:
        sys.stdout.write(render(yaml.safe_load(source), previous))
