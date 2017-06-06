package main

var schema = string(`
{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Moby Config",
  "additionalProperties": false,
  "definitions": {
    "kernel": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "image": { "type": "string"},
        "cmdline": { "type": "string"}
      }
    },
    "file": {
      "type": "object",
      "additionalProperties": false,
        "properties": {
          "path": {"type": "string"},
          "directory": {"type": "boolean"},
          "contents": {"type": "string"}
        }
    },
    "files": {
        "type": "array", 
        "items": { "$ref": "#/definitions/file" }
    },
    "output": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "format": {"type": "string"},
        "project": {"type": "string"},
        "bucket": {"type": "string"},
        "family": {"type": "string"},
        "keys": {"type": "string"},
        "public": {"type": "boolean"},
        "replace": {"type": "boolean"}
      }
    },
    "outputs": {
        "type": "array", 
        "items": { "$ref": "#/definitions/output" }
    },
    "trust": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "image": { "$ref": "#/definitions/strings" },
        "org": { "$ref": "#/definitions/strings" }
      }
    },
    "strings": {
        "type": "array", 
        "items": {"type": "string"}
    },
    "mount": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "destination": { "type": "string" },
        "type": { "type": "string" },
        "source": { "type": "string" },
        "options": { "$ref": "#/definitions/strings" }
      }
    },
    "mounts": {
      "type": "array",
      "items": { "$ref": "#/definitions/mount" }
    },
    "image": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name", "image"],
      "properties": {
        "name": {"type": "string"},
        "image": {"type": "string"},
        "capabilities": { "$ref": "#/definitions/strings" },
        "mounts": { "$ref": "#/definitions/mounts" },
        "binds": { "$ref": "#/definitions/strings" },
        "tmpfs": { "$ref": "#/definitions/strings" },
        "command": { "$ref": "#/definitions/strings" },
        "env": { "$ref": "#/definitions/strings" },
        "cwd": { "type": "string"},
        "net": { "type": "string"},
        "pid": { "type": "string"},
        "ipc": { "type": "string"},
        "uts": { "type": "string"},
        "readonly": { "type": "boolean"},
        "maskedPaths": { "$ref": "#/definitions/strings" },
        "readonlyPaths": { "$ref": "#/definitions/strings" },
        "uid": {"type": "integer"},
        "gid": {"type": "integer"},
        "additionalGids": {
            "type": "array", 
            "items": { "type": "integer" }
        },
        "noNewPrivileges": {"type": "boolean"},
        "hostname": {"type": "string"},
        "oomScoreAdj": {"type": "integer"},
        "disableOOMKiller": {"type": "boolean"},
        "rootfsPropagation": {"type": "string"},
        "cgroupsPath": {"type": "string"},
        "sysctl": {
            "type": "array", 
            "items": { "$ref": "#/definitions/strings" }
        }
      }
    },
    "images": {
        "type": "array", 
        "items": { "$ref": "#/definitions/image" }
    }
  },
  "properties": {
    "kernel": { "$ref": "#/definitions/kernel" },  
    "init": { "$ref": "#/definitions/strings" },
    "onboot": { "$ref": "#/definitions/images" },
    "services": { "$ref": "#/definitions/images" },
    "trust": { "$ref": "#/definitions/trust" },
    "files": { "$ref": "#/definitions/files" },
    "outputs": { "$ref": "#/definitions/outputs" }
  }
}
`)
