# Wisebot Operator

This repo contains the daemon process that manages and controlls the different applications proccesses than runs on the raspberry.

This daemon knows how to update either itself and its processes.

PS: *This is under development*

## Wisebot

### Subscribable topics

#### Current Status

| Topic to Subscribe | Payload |
|:-----:|:---:|
|`/operator/:wisebot-id/healthz`| Empty Payload |

After this event we publish the following message with all the operator's process current information:

**Route**: `/operator/:wisebot-id/healthz:response`

**Expected Payload**:

```json
{
  "data": [
    { "name": "core", "status": "running", "version": "e3b1730", "repo_version": "e3b1730" },
    { "name": "ble", "status": "updating", "version": "db0ba56", "repo_version": "fddc960" },
  ],
  "meta": {
    "wifi_status": { "is_connected": true, "essid": "foo bar house" },
    "mqtt_status": { "is_connected": false }
  }
}
```

#### Update

This task should not replace the current process until the wisebot stop making usage of actionators as water pump, etc.

| Topic to Subscribe | Payload |
|:-----:|:---:|
|`/operator/:wisebot-id/update`| Empty Payload |

#### Start

| Topic to Subscribe | Payload |
|:-----:|:---:|
|`/operator/:wisebot-id/start`| Empty Payload |

#### Stop

| Topic to Subscribe | Payload |
|:-----:|:---:|
|`/operator/:wisebot-id/stop`| Empty Payload |

### Publishable topics

Each of the following topics will publish to **Route**: `/operator/:wisebot-id/healthz:response` after executing.

#### Start Process

**Route**: `/operator/:wisebot-id/start`

**Expected Payload**:

```js
{
  "process": { "name": "core" }
}
```

#### Stop Process

**Route**: `/operator/:wisebot-id/stop`

**Expected Payload**:

```js
{
  "process": { "name": "core" }
}
```

#### Updating Process

**Route**: `/operator/:wisebot-id/process-update`

**Expected Payload**:

```js
{
  "process": { "name": "core" }
}
```

------

## TODO

- [X] Connect to aws iot
- [ ] Update itself
- [X] Update core process
- [ ] Connect with BLE server/service
