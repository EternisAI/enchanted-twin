// @generated
// This file is auto-generated at build time from runtime-dependencies.json
// Do not edit manually!

export const EMBEDDED_RUNTIME_DEPS_CONFIG = {
  "dependencies": {
    "postgres": {
      "backend_download_enabled": true,
      "url": {
        "darwin-arm64": "https://d1vu5azmz7om3b.cloudfront.net/enchanted_data/postgres",
        "darwin-x64": "https://d1vu5azmz7om3b.cloudfront.net/enchanted_data/postgres",
        "linux-x64": "https://d1vu5azmz7om3b.cloudfront.net/enchanted_data/postgres-linux-debian.txz",
        "linux-arm64": "https://d1vu5azmz7om3b.cloudfront.net/enchanted_data/postgres-linux-debian-arm64.txz"
      },
      "name": "postgres",
      "display_name": "PostgreSQL database",
      "description": "Vector database for memory storage",
      "category": "infrastructure",
      "dir": "{DEPENDENCIES_DIR}/postgres",
      "type": "platform_mixed",
      "platform_url_key": true,
      "validation_files": {
        "darwin-arm64": [
          "bin/postgres",
          "bin/initdb",
          "bin/pg_ctl",
          "lib/libpq.5.dylib",
          "lib/postgresql/vector.dylib",
          "lib/libicuuc.75.dylib",
          "lib/libssl.3.dylib",
          "lib/libcrypto.3.dylib",
          "share/postgresql/postgres.bki",
          "share/postgresql/pg_hba.conf.sample",
          "share/postgresql/postgresql.conf.sample",
          "share/postgresql/extension/vector.control",
          "share/postgresql/extension/plpgsql.control"
        ],
        "darwin-x64": [
          "bin/postgres",
          "bin/initdb",
          "bin/pg_ctl",
          "lib/libpq.5.dylib",
          "lib/postgresql/vector.dylib",
          "lib/libicuuc.75.dylib",
          "lib/libssl.3.dylib",
          "lib/libcrypto.3.dylib",
          "share/postgresql/postgres.bki",
          "share/postgresql/pg_hba.conf.sample",
          "share/postgresql/postgresql.conf.sample",
          "share/postgresql/extension/vector.control",
          "share/postgresql/extension/plpgsql.control"
        ],
        "linux-x64": [
          "bin/postgres",
          "bin/initdb",
          "bin/pg_ctl",
          "lib/libpq.so.5",
          "lib/postgresql/vector.so",
          "lib/libicuuc.so.60",
          "lib/libssl.so.1.1",
          "lib/libcrypto.so.1.1",
          "share/postgresql/postgres.bki",
          "share/postgresql/pg_hba.conf.sample",
          "share/postgresql/postgresql.conf.sample",
          "share/postgresql/extension/vector.control",
          "share/postgresql/extension/plpgsql.control"
        ],
        "linux-arm64": [
          "bin/postgres",
          "bin/initdb",
          "bin/pg_ctl",
          "lib/libpq.so.5",
          "lib/postgresql/vector.so",
          "lib/libicuuc.so.60",
          "lib/libssl.so.1.1",
          "lib/libcrypto.so.1.1",
          "share/postgresql/postgres.bki",
          "share/postgresql/pg_hba.conf.sample",
          "share/postgresql/postgresql.conf.sample",
          "share/postgresql/extension/vector.control",
          "share/postgresql/extension/plpgsql.control"
        ]
      },
      "platform_config": {
        "darwin-arm64": {
          "type": "individual_files",
          "files": {
            "binaries": [
              "bin/postgres",
              "bin/initdb",
              "bin/pg_ctl"
            ],
            "libraries": [
              "lib/libpq.5.dylib",
              "lib/libpq.dylib",
              "lib/postgresql/vector.dylib",
              "lib/postgresql/dict_snowball.dylib",
              "lib/postgresql/plpgsql.dylib",
              "lib/libicuuc.75.dylib",
              "lib/libicui18n.75.dylib",
              "lib/libicudata.75.dylib",
              "lib/libssl.3.dylib",
              "lib/libcrypto.3.dylib",
              "lib/libxml2.2.dylib",
              "lib/libzstd.1.dylib",
              "lib/liblz4.1.dylib"
            ],
            "dataFiles": [
              "share/postgresql/postgres.bki",
              "share/postgresql/errcodes.txt",
              "share/postgresql/information_schema.sql",
              "share/postgresql/pg_hba.conf.sample",
              "share/postgresql/pg_ident.conf.sample",
              "share/postgresql/postgresql.conf.sample",
              "share/postgresql/pg_service.conf.sample",
              "share/postgresql/psqlrc.sample",
              "share/postgresql/system_constraints.sql",
              "share/postgresql/system_functions.sql",
              "share/postgresql/system_views.sql",
              "share/postgresql/sql_features.txt",
              "share/postgresql/snowball_create.sql",
              "share/postgresql/extension/plpgsql.control",
              "share/postgresql/extension/plpgsql--1.0.sql",
              "share/postgresql/extension/vector.control",
              "share/postgresql/extension/vector--0.8.0.sql",
              "share/postgresql/timezone/UTC",
              "share/postgresql/timezonesets/Africa.txt",
              "share/postgresql/timezonesets/America.txt",
              "share/postgresql/timezonesets/Antarctica.txt",
              "share/postgresql/timezonesets/Asia.txt",
              "share/postgresql/timezonesets/Atlantic.txt",
              "share/postgresql/timezonesets/Australia",
              "share/postgresql/timezonesets/Australia.txt",
              "share/postgresql/timezonesets/Default",
              "share/postgresql/timezonesets/Etc.txt",
              "share/postgresql/timezonesets/Europe.txt",
              "share/postgresql/timezonesets/India",
              "share/postgresql/timezonesets/Indian.txt",
              "share/postgresql/timezonesets/Pacific.txt",
              "share/postgresql/tsearch_data/english.stop"
            ]
          }
        },
        "darwin-x64": {
          "type": "individual_files",
          "files": {
            "binaries": [
              "bin/postgres",
              "bin/initdb",
              "bin/pg_ctl"
            ],
            "libraries": [
              "lib/libpq.5.dylib",
              "lib/libpq.dylib",
              "lib/postgresql/vector.dylib",
              "lib/postgresql/dict_snowball.dylib",
              "lib/postgresql/plpgsql.dylib",
              "lib/libicuuc.75.dylib",
              "lib/libicui18n.75.dylib",
              "lib/libicudata.75.dylib",
              "lib/libssl.3.dylib",
              "lib/libcrypto.3.dylib",
              "lib/libxml2.2.dylib",
              "lib/libzstd.1.dylib",
              "lib/liblz4.1.dylib"
            ],
            "dataFiles": [
              "share/postgresql/postgres.bki",
              "share/postgresql/errcodes.txt",
              "share/postgresql/information_schema.sql",
              "share/postgresql/pg_hba.conf.sample",
              "share/postgresql/pg_ident.conf.sample",
              "share/postgresql/postgresql.conf.sample",
              "share/postgresql/pg_service.conf.sample",
              "share/postgresql/psqlrc.sample",
              "share/postgresql/system_constraints.sql",
              "share/postgresql/system_functions.sql",
              "share/postgresql/system_views.sql",
              "share/postgresql/sql_features.txt",
              "share/postgresql/snowball_create.sql",
              "share/postgresql/extension/plpgsql.control",
              "share/postgresql/extension/plpgsql--1.0.sql",
              "share/postgresql/extension/vector.control",
              "share/postgresql/extension/vector--0.8.0.sql",
              "share/postgresql/timezone/UTC",
              "share/postgresql/timezonesets/Africa.txt",
              "share/postgresql/timezonesets/America.txt",
              "share/postgresql/timezonesets/Antarctica.txt",
              "share/postgresql/timezonesets/Asia.txt",
              "share/postgresql/timezonesets/Atlantic.txt",
              "share/postgresql/timezonesets/Australia",
              "share/postgresql/timezonesets/Australia.txt",
              "share/postgresql/timezonesets/Default",
              "share/postgresql/timezonesets/Etc.txt",
              "share/postgresql/timezonesets/Europe.txt",
              "share/postgresql/timezonesets/India",
              "share/postgresql/timezonesets/Indian.txt",
              "share/postgresql/timezonesets/Pacific.txt",
              "share/postgresql/tsearch_data/english.stop"
            ]
          }
        },
        "linux-x64": {
          "type": "tar.xz"
        },
        "linux-arm64": {
          "type": "tar.xz"
        }
      },
      "post_download": {
        "chmod": {
          "files": [
            "bin/postgres",
            "bin/initdb",
            "bin/pg_ctl"
          ],
          "mode": "755"
        }
      }
    },
    "embeddings": {
      "backend_download_enabled": true,
      "url": "https://d3o88a4htgfnky.cloudfront.net/models/jina-embeddings-v2-base-en.zip",
      "name": "embeddings",
      "display_name": "Embeddings model",
      "description": "Enchanted makes sense of your content",
      "category": "model",
      "dir": "{DEPENDENCIES_DIR}/models/jina-embeddings-v2-base-en",
      "type": "zip",
      "validation_files": [
        "model.onnx"
      ],
      "post_download": {
        "cleanup": [
          "*.zip"
        ]
      }
    },
    "anonymizer": {
      "backend_download_enabled": true,
      "url": "https://d3o88a4htgfnky.cloudfront.net/models/qwen3-4b_q4_k_m.zip",
      "name": "anonymizer",
      "display_name": "Anonymizer model",
      "description": "Enchanted keeps your data private",
      "category": "model",
      "dir": "{DEPENDENCIES_DIR}/models/anonymizer",
      "type": "zip",
      "validation_files": [
        "qwen3-0.6b-q4_k_m.gguf",
        "qwen3-4b_q4_k_m.gguf"
      ],
      "post_download": {
        "cleanup": [
          "*.zip"
        ]
      }
    },
    "onnx": {
      "backend_download_enabled": true,
      "url": {
        "darwin-arm64": "https://d3o88a4htgfnky.cloudfront.net/assets/onnxruntime-osx-arm64-1.22.0.tgz",
        "linux-x64": "https://d3o88a4htgfnky.cloudfront.net/assets/onnxruntime-linux-x64-1.22.0.tgz",
        "linux-arm64": "https://d1vu5azmz7om3b.cloudfront.net/enchanted_data/onnxruntime-linux-aarch64-1.22.0.tgz"
      },
      "name": "onnx",
      "display_name": "Inference engine",
      "description": "",
      "category": "infrastructure",
      "dir": "{DEPENDENCIES_DIR}/shared/lib",
      "type": "tar.gz",
      "platform_url_key": true,
      "validation_files": {
        "darwin-arm64": [
          "onnxruntime-osx-arm64-1.22.0/lib/libonnxruntime.dylib"
        ],
        "linux-x64": [
          "onnxruntime-linux-x64-1.22.0/lib/libonnxruntime.so"
        ],
        "linux-arm64": [
          "onnxruntime-linux-aarch64-1.22.0/lib/libonnxruntime.so"
        ]
      },
      "post_download": {
        "cleanup": [
          "*.tgz"
        ]
      }
    },
    "llamaccp": {
      "backend_download_enabled": true,
      "url": "https://github.com/ggml-org/llama.cpp/releases/download/b5916/llama-b5916-bin-macos-arm64.zip",
      "name": "llamaccp",
      "display_name": "LLM engine",
      "description": "",
      "category": "infrastructure",
      "dir": "{DEPENDENCIES_DIR}/shared/lib/llamaccp",
      "type": "zip",
      "validation_files": [
        "build"
      ],
      "post_download": {
        "cleanup": [
          "*.zip"
        ]
      }
    },
    "uv": {
      "backend_download_enabled": false,
      "url": "",
      "name": "uv",
      "display_name": "Voice mode dependencies",
      "description": "",
      "category": "infrastructure",
      "dir": "{DEPENDENCIES_DIR}/shared/bin",
      "type": "curl_script",
      "install_script": "curl -LsSf https://astral.sh/uv/install.sh | sh",
      "validation_files": {
        "win32": [
          "uv.exe"
        ],
        "default": [
          "uv"
        ]
      },
      "validation_condition": "platform_specific_binary",
      "post_download": {
        "copy_from_global": true
      }
    }
  },
  "base_paths": {
    "DEPENDENCIES_DIR": "{app.getPath('appData')}/enchanted"
  },
  "platform_mappings": {
    "darwin": "macos",
    "linux": "linux",
    "win32": "windows"
  }
} as const;
