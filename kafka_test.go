package kafka

import "testing"

var noopts = map[string]string{}


func Test_read_route_address(t *testing.T) {
	address := "broker1:9092,broker2:9092"
	brokers := readBrokers(address)
	if len(brokers) != 2 {
		t.Fatal("expected two broker addrs")
	}
	if brokers[0] != "broker1:9092" {
		t.Errorf("broker1 addr should not be %s", brokers[0])
	}
	if brokers[1] != "broker2:9092" {
		t.Errorf("broker2 addr should not be %s", brokers[1])
	}
}

func Test_read_route_address_with_a_slash_topic(t *testing.T) {
	address := "broker/hello"
	brokers := readBrokers(address)
	if len(brokers) != 1 {
		t.Fatal("expected a broker addr")
	}

	topic := readTopic(address, noopts)
	if topic != "hello" {
		t.Errorf("topic should not be %s", topic)
	}
}

func Test_read_topic_option(t *testing.T) {
	opts := map[string]string{"topic": "hello"}
	topic := readTopic("", opts)
	if topic != "hello" {
		t.Errorf("topic should not be %s", topic)
	}
}

func Test_read_route_address_with_a_slash_topic_trumps_a_topic_option(t *testing.T) {
	opts := map[string]string{"topic": "trumped"}
	topic := readTopic("broker/hello", opts)
	if topic != "hello" {
		t.Errorf("topic should not be %s", topic)
	}
}
