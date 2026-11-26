---
applyTo: "api/**,client/**,daemon/server/**"
---

# Review API changes

## API version

New API behavior must go into the next unreleased API version.

- The current version is `MaxAPIVersion` in `daemon/config/config.go`.
  If `api/docs/v<MaxAPIVersion>.yaml` already exists, that version has been released and new changes belong in the next one.
- Check that the version is consistent in every place the PR uses it: the `## v<X> API changes` section of `api/docs/CHANGELOG.md`, version gates such as `versions.LessThan(version, "<X>")` in `daemon/server/router`, and the API definition.
- Changes to existing behavior need a version gate so older clients keep the old behavior.

## API definition and types

- `api/openapi.yaml` is the source of truth.
  `api/swagger.yaml` is generated with `make -C api swagger-spec` and must not be edited by hand; flag PRs that change one without the other.
- New fields in `api/types` need explicit JSON tags with the intended casing (`ID`, not `Id`), and `omitzero` or `omitempty` for optional fields.
- For new fields, flag designs where nil, empty, and populated values mean different things without that being documented; ask for an explicit field or type instead.
  Do not flag documented semantics of existing fields, such as `POST /containers/{id}/update` treating an omitted field as unchanged and an empty array as clearing it.
- New fields in the container `Config` must also be handled by the image config comparison in `daemon/internal/image/cache/compare.go`, which the classic builder cache uses.
- In the client, prefer typed results (like `ImagePullResponse` in `client/image_pull.go`) over callbacks or generic JSON progress messages, and pass data through options structs rather than through `context.Context`.
