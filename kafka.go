package kafka

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"
	"crypto/tls"
	"io/ioutil"

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
	brokers := readBrokers(route.Address)
	if len(brokers) == 0 {
		return nil, errorf("The Kafka broker host:port is missing. Did you specify it as a route address?")
	}

	topic := readTopic(route.Address, route.Options)
	if topic == "" {
		return nil, errorf("The Kafka topic is missing. Did you specify it as a route option?")
	}

	var err error

	cert_file := os.Getenv("TLS_CERT_FILE")
	key_file := os.Getenv("TLS_PRIVKEY_FILE")

	var tmpl *template.Template
	if text := os.Getenv("KAFKA_TEMPLATE"); text != "" {
		tmpl, err = template.New("kafka").Parse(text)
		if err != nil {
			return nil, errorf("Couldn't parse Kafka message template. %v", err)
		}
	}

	if os.Getenv("DEBUG") != "" {
		log.Printf("Starting Kafka producer for address: %s, topic: %s.\n", brokers, topic)
	}

	var retries int
	retries, err = strconv.Atoi(os.Getenv("KAFKA_CONNECT_RETRIES"))
	if err != nil {
		retries = 3
	}

	var producer sarama.AsyncProducer

	if os.Getenv("DEBUG") != "" {
		log.Println("Generating Kafka configuration.")
	}
  config := newConfig()

	if (cert_file != "") && (key_file != "") {
		if os.Getenv("DEBUG") != "" {
			log.Println("Enabling Kafka TLS support.")
		}

		certfile, err := os.Open(cert_file)
		if err != nil {
			return nil, errorf("Couldn't open TLS certificate file: %s", err)
		}

		keyfile, err := os.Open(key_file)
		if err != nil {
			return nil, errorf("Couldn't open TLS private key file: %s", err)
		}

		tls_cert, err := ioutil.ReadAll(certfile)
		if err != nil {
			return nil, errorf("Couldn't read TLS certificate: %s", err)
		}

		tls_privkey, err := ioutil.ReadAll(keyfile)
		if err != nil {
			return nil, errorf("Couldn't read TLS private key: %s", err)
		}

		keypair, err := tls.X509KeyPair([]byte(tls_cert), []byte(tls_privkey))
		if err != nil {
			return nil, errorf("Couldn't establish TLS authentication keypair. Check TLS_CERT and TLS_PRIVKEY environment vars.")
		}

		tls_configuration := &tls.Config{
			Certificates:	[]tls.Certificate{keypair},
			InsecureSkipVerify: false,
		}

		config.Net.TLS.Config = tls_configuration
		config.Net.TLS.Enable = true
	}

	for i := 0; i < retries; i++ {

		producer, err = sarama.NewAsyncProducer(brokers, config)

		if err != nil {
			if os.Getenv("DEBUG") != "" {
				log.Println("Couldn't create Kafka producer. Retrying...", err)
			}
			if i == retries-1 {
				return nil, errorf("Couldn't create Kafka producer. %v", err)
			}
		} else {
			time.Sleep(1 * time.Second)
		}
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

func readBrokers(address string) []string {
	if strings.Contains(address, "/") {
		slash := strings.Index(address, "/")
		address = address[:slash]
	}

	return strings.Split(address, ",")
}

func readTopic(address string, options map[string]string) string {
	var topic string
	if !strings.Contains(address, "/") {
		topic = options["topic"]
	} else {
		slash := strings.Index(address, "/")
		topic = address[slash+1:]
	}

	return topic
}

func errorf(format string, a ...interface{}) (err error) {
	err = fmt.Errorf(format, a...)
	if os.Getenv("DEBUG") != "" {
		fmt.Println(err.Error())
	}
	return
}
