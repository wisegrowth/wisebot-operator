# Wisebot Operator

This repo contains the daemon process that manages and controlls the different applications proccesses than runs on the raspberry.

This daemon knows how to update either itself and its processes.

PS: *This is under development*

## Wisebot

### Subscribable topics

#### Healthz - Current Status

The operator will subscribe to the following topic:

| Topic | Payload |
|:-----:|:---:|
|`/operator/:wisebot-id/healthz`| Empty Payload |

In order to react to this event, the operator will publish the following message with all the operator's processes current information:

**Route**: `/operator/:wisebot-id/healthz:response`

**Message Payload**:

```json
{
  "data": {
    "services": [
      { "name": "core", "status": "running", "version": "e3b1730", "repo_version": "e3b1730" },
      { "name": "ble", "status": "updating", "version": "db0ba56", "repo_version": "fddc960" }
    ],
    "daemons": [
      { "name": "led", "status": "running", "repo_version": "e3b1730" },
      { "name": "filebeat", "status": "running", "repo_version": "" }
    ],
  },
  "meta": {
    "wifi_status": { "is_connected": true, "essid": "foo bar house" },
    "mqtt_status": { "is_connected": false }
  }
}
```

### Publishable topics

THe operator will be listening the following topics.

> Each of the following topics will publish to **Route**: `/operator/:wisebot-id/healthz:response` after executing.

#### Start Service

**Route**: `/operator/:wisebot-id/service-start`

**Expected Payload**:

```js
{
  "name": "core"
}
```

#### Stop Service

**Route**: `/operator/:wisebot-id/service-stop`

**Expected Payload**:

```js
{
  "name": "core"
}
```

#### Update Service

**Route**: `/operator/:wisebot-id/service-update`

**Expected Payload**:

```js
{
  "name": "core"
}
```

#### Start Daemon

**Route**: `/operator/:wisebot-id/daemon-start`

**Expected Payload**:

```js
{
  "name": "core"
}
```

#### Stop Daemon

**Route**: `/operator/:wisebot-id/daemon-stop`

**Expected Payload**:

```js
{
  "name": "core"
}
```

#### Restart Daemon

**Route**: `/operator/:wisebot-id/daemon-restart`

**Expected Payload**:

```js
{
  "name": "core"
}
```

#### Update Daemon

**Route**: `/operator/:wisebot-id/daemon-update`

**Expected Payload**:

```js
{
  "name": "core"
}
```
------

## TODO

- [X] Connect to aws iot
- [ ] Update itself
- [X] Update core process
- [X] Connect with BLE server/service
