"""Project Engine OpenAPI schemas for the Swagger 2 compatibility specification."""


REF_PREFIX = "#/components/schemas/"
OPENAPI_VERSION = "3.2.0"
# JSON Schema 2020-12 keywords with no Swagger 2 equivalent. Nullability is
# expressed only through the x-nullable generator extension, so the schemas
# must not use type unions or the removed OpenAPI 3.0 nullable keyword.
UNSUPPORTED = (
    "anyOf",
    "oneOf",
    "not",
    "const",
    "examples",
    "$defs",
    "discriminator",
    "nullable",
    "writeOnly",
    "deprecated",
)


def project_schema(schema):
    """Preserve schema order and opaque examples/extensions while rewriting refs."""
    if isinstance(schema, bool):
        return schema  # additionalProperties
    result = dict(schema)
    for keyword in UNSUPPORTED:
        if keyword in result:
            raise ValueError(f"model projection does not support {keyword}")
    if isinstance(result.get("type"), list):
        raise ValueError("model projection does not support type unions")
    if "$ref" in result:
        ref = result["$ref"]
        if not ref.startswith(REF_PREFIX):
            raise ValueError(f"model projection requires a local schema reference: {ref}")
        # Swagger 2 tooling (go-swagger) honors the description and generator
        # extensions placed beside $ref, as JSON Schema 2020-12 does.
        result["$ref"] = "#/definitions/" + ref[len(REF_PREFIX):]
    if "properties" in result:
        result["properties"] = {
            name: project_schema(value) for name, value in result["properties"].items()
        }
    for keyword in ("items", "additionalProperties"):
        if keyword in result:
            result[keyword] = project_schema(result[keyword])
    if "allOf" in result:
        result["allOf"] = [project_schema(value) for value in result["allOf"]]
    return result


def project(spec):
    if spec.get("openapi") != OPENAPI_VERSION:
        raise ValueError(f"model projection requires OpenAPI {OPENAPI_VERSION}")
    return {
        "swagger": "2.0",
        "info": spec["info"],
        "paths": {},
        "definitions": {
            name: project_schema(schema)
            for name, schema in spec["components"]["schemas"].items()
        },
    }
