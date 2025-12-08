package testutil

// Sample JSON responses for testing ZFS commands

const ZFSListJSON = `{
  "output_version": {
    "command": "zfs list",
    "vers_major": 0,
    "vers_minor": 1
  },
  "datasets": {
    "tank": {
      "name": "tank",
      "type": "FILESYSTEM",
      "pool": "tank",
      "createtxg": "1",
      "properties": {
        "mountpoint": {
          "value": "/tank",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "used": {
          "value": "1024000",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "available": {
          "value": "2048000",
          "source": {
            "type": "default", 
            "data": ""
          }
        },
        "referenced": {
          "value": "512000",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "compressratio": {
          "value": "1.50x",
          "source": {
            "type": "default",
            "data": ""
          }
        }
      }
    },
    "tank/data": {
      "name": "tank/data",
      "type": "FILESYSTEM", 
      "pool": "tank",
      "createtxg": "2",
      "properties": {
        "mountpoint": {
          "value": "/tank/data",
          "source": {
            "type": "inherited",
            "data": "tank"
          }
        },
        "used": {
          "value": "512000",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "available": {
          "value": "1536000",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "referenced": {
          "value": "256000",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "compressratio": {
          "value": "2.00x",
          "source": {
            "type": "default",
            "data": ""
          }
        }
      }
    }
  }
}`

const ZFSGetJSON = `{
  "output_version": {
    "command": "zfs get",
    "vers_major": 0,
    "vers_minor": 1
  },
  "datasets": {
    "tank": {
      "name": "tank",
      "type": "FILESYSTEM",
      "pool": "tank",
      "createtxg": "1",
      "properties": {
        "compression": {
          "value": "lz4",
          "source": {
            "type": "local",
            "data": ""
          }
        }
      }
    }
  }
}`

const ZPoolListJSON = `{
  "output_version": {
    "command": "zpool list",
    "vers_major": 0,
    "vers_minor": 1
  },
  "pools": {
    "tank": {
      "name": "tank",
      "type": "",
      "state": "ONLINE",
      "pool_guid": "12345678901234567890",
      "txg": "100",
      "spa_version": "5000",
      "zpl_version": "5",
      "properties": {
        "size": {
          "value": "10G",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "free": {
          "value": "8G",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "allocated": {
          "value": "2G",
          "source": {
            "type": "default", 
            "data": ""
          }
        }
      }
    }
  }
}`

const ZDBOutput = `version: 5000
name: tank
	type: root
		id: 0
		guid: 12345678901234567890
		path: /dev/ada0p3
		whole_disk: 0
		metaslab_array: 71
		metaslab_shift: 24
		ashift: 9
		asize: 10737418240
		is_log: 0
		create_txg: 4`
