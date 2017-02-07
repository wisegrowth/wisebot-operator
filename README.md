# Wisebot Operator

This repo contains the daemon process that manages and controlls
the different applications proccesses than runs on the raspberry.

This daemon knows how to update either itself and its processes.

PS: *This is under development*

## Wisebot

### Subscribable topics

#### Current Status

| Topic to Subscribe | Payload |
|:-----:|:---:|
|`/operator/:wisebot-id/healthz`| Empty Payload |

After this event we must publish the following message with
all the operator's process current information:

**Route**: `/operator/:wisebot-id/healthz:response`

**Expected Payload**:

```json
{
  "data": [
    { "name": "wisebot", "status": "running", "version": "1.0.1" },
    { "name": "ble", "status": "updating", "version": "2.0.4" }
  ]
}
```

#### Update

This task should not replace the current process until the wisebot
stop making usage of actionators as water pump, etc.

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

#### Updating Process

**Route**: `/operator/:wisebot-id/process-update`

**Expected Payload**:

```js
{
  "data": {
    "name": "wisebot",
    "status": "update:started", // update:started - update:finished
    "new_version": "5c38683",
    "old_version": "dfd09a7"
  },
  "meta": {
    "proccesses": [
      { "name": "wisebot", "status": "running", "version": "1.0.1" },
      { "name": "ble", "status": "updating", "version": "2.0.4" }
    ]
  }
}
```

------

## TODO

- [X] Connect to aws iot
- [ ] Update itself
- [ ] Update wisebot process
- [ ] Connect with BLE server/service
