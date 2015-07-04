package main

import (
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	region   string
	topicARN = kingpin.Arg("topic", "SNS Topic ARN").Required().String()
	debug    = kingpin.Flag("debug", "Enable debug mode").Short('d').Bool()

	messagesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hermes_received_messages_total",
		Help: "Number of messages processd by Hermes",
	})
	messagesErrored = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hermes_error_alerts_total",
		Help: "Number of messages received by Hermes that ended in an error",
	})
)

func init() {
	kingpin.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	inflectRegionFromARN(*topicARN)
	log.WithFields(log.Fields{
		"region":   region,
		"topicARN": *topicARN,
		"debug":    *debug,
	}).Info("starting webhook service")

	prometheus.MustRegister(messagesReceived)
	prometheus.MustRegister(messagesErrored)
}

func main() {
	r := gin.Default()
	r.POST("/event", func(c *gin.Context) {
		messagesReceived.Inc()

		defer c.Request.Body.Close()
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			log.WithFields(log.Fields{
				"origErr": err.Error(),
			}).Error("could not read request body")
			messagesErrored.Inc()
			return
		}

		if *debug {
			log.WithFields(log.Fields{
				"client": c.ClientIP(),
				"body":   string(body),
			}).Debug("received request")
		}

		err = forwardToSNS(body)

		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				log.WithFields(log.Fields{
					"code":    awsErr.Code(),
					"origErr": awsErr.OrigErr(),
				}).Error(awsErr.Message())
				if reqErr, ok := err.(awserr.RequestFailure); ok {
					log.WithFields(log.Fields{
						"code":       reqErr.Code(),
						"statusCode": reqErr.StatusCode(),
						"requestId":  reqErr.RequestID(),
					}).Error(reqErr.Message())
					c.String(http.StatusInternalServerError, "ERR")
					messagesErrored.Inc()
					return
				}
				c.String(http.StatusBadGateway, "ERR")
			} else {
				log.Error(err.Error())
				c.String(http.StatusInternalServerError, "ERR")
			}
			messagesErrored.Inc()
			return
		}

		log.WithFields(log.Fields{
			"body": string(body),
		}).Info("successfully forwarded message")
		c.String(http.StatusNoContent, "")
	})

	r.GET("/metrics", gin.WrapH(prometheus.Handler()))

	r.Run(":8080")
}

func inflectRegionFromARN(arn string) {
	parts := strings.Split(arn, ":")
	if len(parts) != 6 {
		log.Fatal("Could not inflect AWS region from ARN: ARN does not look valid")
	}
	region = string(parts[3])
}

func forwardToSNS(data []byte) error {
	svc := sns.New(&aws.Config{Region: region})

	params := &sns.PublishInput{
		Message:  aws.String(string(data)),
		TopicARN: aws.String(*topicARN),
	}
	_, err := svc.Publish(params)
	return err
}
