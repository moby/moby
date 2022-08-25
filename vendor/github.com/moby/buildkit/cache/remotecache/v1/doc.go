package cacheimport

// Distributable build cache
//
// Main manifest is OCI image index
// https://github.com/opencontainers/image-spec/blob/master/image-index.md .
// Manifests array contains descriptors to the cache layers and one instance of
// build cache config with media type application/vnd.buildkit.cacheconfig.v0 .
// The cache layer descriptors need to have an annotation with uncompressed digest
// to allow deduplication on extraction and optionally "buildkit/createdat"
// annotation to support maintaining original timestamps.
//
// Cache config file layout:
//
//{
//  "layers": [                       <- layers contains references to blobs
//    {
//      "blob": "sha256:deadbeef",    <- digest of layer blob in index
//      "parent": -1                  <- index of parent layer, -1 if no parent
//    },
//    {
//      "blob": "sha256:deadbeef",
//      "parent": 0
//    }
//  ],
//
//  "records": [                       <- records contains chains of cache keys
//    {
//      "digest": "sha256:deadbeef",   <- base digest for the record
//    },
//    {
//      "digest": "sha256:deadbeef",
//      "output": 1,                   <- optional output index
//      "layers": [                    <- optional array of layer pointers
//        {
//          "createdat": "",
//          "layer": 1,                <- index to the layers array, layer is loaded with all of its parents
//        }
//      ],
//      "chains": [                    <- optional array of layer pointer lists
//        {
//          "createdat": "",
//          "layers": [1],             <- indexes to the layers array, all layers are loaded in specified order without parents
//        }
//      ],
//      "inputs": [                    <- dependant records, this is how cache keys are linked together
//        [                            <- index of the dependency (0)
//          {
//            "selector": "sel",       <- optional selector
//            "link": 0,               <- index to the dependant record
//          }
//        ]
//      ]
//    }
//  ]
// }
