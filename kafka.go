package kafka

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/gliderlabs/logspout/router"
	"gopkg.in/Shopify/sarama.v1"
)

func init() {
	router.AdapterFactories.Register(NewKafkaAdapter, "kafka")
}

type KafkaAdapter struct {
	route    *router.Route
	brokers  []string
	topic    string
	producer sarama.AsyncProducer
	tmpl     *template.Template
}

func NewKafkaAdapter(route *router.Route) (router.LogAdapter, error) {
	brokers, topic, err := parseRouteAddress(route.Address)
	if err != nil {
		return nil, err
	}

	var tmpl *template.Template
	if text := os.Getenv("KAFKA_TEMPLATE"); text != "" {
		tmpl, err = template.New("kafka").Parse(text)
		if err != nil {
			return nil, err
		}
	}

	if os.Getenv("DEBUG") != "" {
		log.Printf("Starting Kafka producer for address %s\n", route.Address)
	}

	producer, err := sarama.NewAsyncProducer(brokers, newConfig())
	if err != nil {
		err = fmt.Errorf("Couldn't create Kafka producer. %v", err)
		if os.Getenv("DEBUG") != "" {
			log.Printf(err.Error())
		}

		return nil, err
	}

	return &KafkaAdapter{
		route:    route,
		brokers:  brokers,
		topic:    topic,
		producer: producer,
		tmpl:     tmpl,
	}, nil
}

func (a *KafkaAdapter) Stream(logstream chan *router.Message) {
	defer a.producer.Close()
	for rm := range logstream {
		message, err := a.formatMessage(rm)
		if err != nil {
			log.Println("kafka:", err)
			a.route.Close()
			break
		}

		a.producer.Input() <- message
	}
}

func newConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.ClientID = "logspout"
	config.Producer.Return.Errors = false
	config.Producer.Return.Successes = false
	config.Producer.Flush.Frequency = 1 * time.Second
	config.Producer.RequiredAcks = sarama.WaitForLocal

	if opt := os.Getenv("KAFKA_COMPRESSION_CODEC"); opt != "" {
		switch opt {
		case "gzip":
			config.Producer.Compression = sarama.CompressionGZIP
		case "snappy":
			config.Producer.Compression = sarama.CompressionSnappy
		}
	}

	return config
}

func (a *KafkaAdapter) formatMessage(message *router.Message) (*sarama.ProducerMessage, error) {
	var encoder sarama.Encoder
	if a.tmpl != nil {
		var w bytes.Buffer
		if err := a.tmpl.Execute(&w, message); err != nil {
			return nil, err
		}
		encoder = sarama.ByteEncoder(w.Bytes())
	} else {
		encoder = sarama.StringEncoder(message.Data)
	}

	return &sarama.ProducerMessage{
		Topic: a.topic,
		Value: encoder,
	}, nil
}

func parseRouteAddress(routeAddress string) ([]string, string, error) {
	if !strings.Contains(routeAddress, "/") {
		return []string{}, "", fmt.Errorf("The route address %s didn't specify the Kafka topic.", routeAddress)
	}

	slash := strings.Index(routeAddress, "/")
	topic := routeAddress[slash+1:]
	addrs := strings.Split(routeAddress[:slash], ",")
	return addrs, topic, nil
}
