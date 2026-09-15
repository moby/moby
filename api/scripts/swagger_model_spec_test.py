"""Regression tests for the Engine schema projection, not a general converter."""

from copy import deepcopy
from pathlib import Path
import unittest

import yaml

from swagger_model_spec import project, project_schema


class ModelSpecTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with (Path(__file__).resolve().parents[1] / "openapi.yaml").open() as source:
            cls.spec = yaml.safe_load(source)
        cls.schemas = cls.spec["components"]["schemas"]

    def test_schema_only_and_ordered(self):
        original = deepcopy(self.spec)
        projected = project(self.spec)
        self.assertEqual(projected["swagger"], "2.0")
        self.assertEqual(projected["paths"], {})
        self.assertEqual(list(projected["definitions"]), list(self.schemas))
        for name, schema in self.schemas.items():
            with self.subTest(schema=name):
                self.assertEqual(
                    list(projected["definitions"][name].get("properties", {})),
                    list(schema.get("properties", {})),
                )
        self.assertEqual(self.spec, original)

    def test_reference_siblings_are_kept(self):
        ipam = self.schemas["Network"]["properties"]["IPAM"]
        self.assertEqual(project_schema(ipam), {
            "description": "The network's IP Address Management.\n",
            "$ref": "#/definitions/IPAM",
            "x-nullable": False,
            "x-omitempty": False,
        })

    def test_nullable_reference_uses_generator_extension_only(self):
        rootfs = self.schemas["Storage"]["properties"]["RootFS"]
        self.assertEqual(project_schema(rootfs), {
            "description": "Information about the storage used for the container's root filesystem.\n",
            "type": "object",
            "x-nullable": True,
            "$ref": "#/definitions/RootFSStorage",
        })

    def test_original_allof_is_kept(self):
        schema = self.schemas["MountPoint"]["properties"]["Type"]
        projected = project_schema(schema)
        self.assertNotIn("$ref", projected)
        self.assertEqual(projected["allOf"], [{"$ref": "#/definitions/MountType"}])
        self.assertEqual(projected["description"], schema["description"])

    def test_examples_and_extensions_are_opaque(self):
        data = {"$ref": "#/components/schemas/NotAReference", "nullable": True}
        schema = {
            "type": "object", "example": data, "default": data,
            "x-custom": data, "additionalProperties": False,
            "properties": {"nullable": {"type": "boolean", "x-nullable": False}},
        }
        projected = project_schema(schema)
        for key in ("example", "default", "x-custom", "additionalProperties"):
            self.assertEqual(projected[key], schema[key])
        self.assertEqual(projected["properties"]["nullable"], {
            "type": "boolean", "x-nullable": False,
        })

    def test_unsupported_schemas_fail(self):
        for schema in (
            {"oneOf": [{"type": "string"}]},
            {"anyOf": [{"$ref": "#/components/schemas/Thing"}, {"type": "null"}]},
            {"type": ["object", "null"]},
            {"type": "object", "nullable": True},
            {"type": "string", "examples": ["a"]},
            {"$ref": "other.yaml#/Thing"},
        ):
            with self.subTest(schema=schema), self.assertRaises(ValueError):
                project_schema(schema)

    def test_requires_pinned_openapi_version(self):
        with self.assertRaises(ValueError):
            project({"openapi": "3.0.3", "info": {}, "components": {"schemas": {}}})


if __name__ == "__main__":
    unittest.main()
