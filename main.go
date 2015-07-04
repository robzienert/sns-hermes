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
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	region   string
	topicARN = kingpin.Arg("queue", "SNS Topic ARN").Required().String()
	debug    = kingpin.Flag("debug", "Enable debug mode").Short('d').Bool()
)

func main() {
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

	r := gin.Default()
	r.POST("/event", func(c *gin.Context) {
		defer c.Request.Body.Close()
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			log.WithFields(log.Fields{
				"origErr": err.Error(),
			}).Error("could not read request body")
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
					return
				}
				c.String(http.StatusBadGateway, "ERR")
			} else {
				log.Error(err.Error())
				c.String(http.StatusInternalServerError, "ERR")
			}
			return
		}

		log.WithFields(log.Fields{
			"body": string(body),
		}).Info("successfully forwarded message")
		c.String(http.StatusNoContent, "")
	})

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
