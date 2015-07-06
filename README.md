# sns-hermes

A simple service that forwards webhook POST requests to SNS.

I use this in integration with [Prometheus' Alertmanager](http://prometheus.io/docs/alerting/alertmanager/)
to send alerts to a queue that's being listened to by alert remediation scripts.

## Usage

```
$ go run main.go --help
usage: main [<flags>] <topic>

Flags:
  --help       Show help.
  -d, --debug  Enable debug mode

Args:
  <topic>  SNS Topic ARN
```
