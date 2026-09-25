**DSSE Type Data Documentation**

This document provides a definition for each field that is not otherwise described in the [dsse
schema](https://github.com/sigstore/rekor/blob/main/pkg/types/dsse/v0.0.1/dsse_v0_0_1_schema.json). This
document also notes any additional information about the values
associated with each field such as the format in which the data is
stored and any necessary transformations.

**How do you identify an object as an DSSE object?**

The "Body" field will include an "dsseObj" field.

**Recognized content types**

- [in-toto
  statements](https://github.com/in-toto/attestation/tree/main/spec#statement)
  are recognized and parsed. The found subject hashes are indexed so
  they can be searched for.

**What data about the envelope is stored in Rekor**

Only the hash of the payload (the content covered by the digital signature inside the envelope), the hash of the entire DSSE envelope (including signatures),
the signature(s) and their corresponding verifying materials (e.g. public key(s) or certificates) are stored.

Even if Rekor is configured to use attestation storage, the entire DSSE envelope will not be stored.
