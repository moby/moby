**in-toto Type Data Documentation**

This document provides a definition for each field that is not otherwise described in the [in-toto schema](https://github.com/sigstore/rekor/blob/main/pkg/types/intoto/v0.0.1/intoto_v0_0_1_schema.json). This document also notes any additional information about the values associated with each field such as the format in which the data is stored and any necessary transformations.

**Attestation:** authenticated, machine-readable metadata about one or more software artifacts. [SLSA definition](https://github.com/slsa-framework/slsa/blob/main/controls/attestations.md)
- The Attestation value ought to be a Base64-encoded JSON object.
- The [in-toto Attestation specification](https://github.com/in-toto/attestation/blob/main/spec/README.md#statement) provides detailed guidance on understanding and parsing this JSON object.

**AttestationType:** Identifies the type of attestation being made, such as a provenance attestation or a vulnerability scan attestation. AttestationType's value, even when prefixed with an http, is not necessarily a working URL.

**How do you identify an object as an in-toto object?**

The "Body" field will include an "IntotoObj" field.
