package transport

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	sarama "github.com/Shopify/sarama"
	"github.com/buger/jsonparser"
	"github.com/prometheus/client_golang/prometheus"
	"wizzie.io/plane/engine/utils"
)

var (
	KafkaTopicIn      *string
	KafkaTopicOut     *string
	KafkaTopicTable   *string
	KafkaTopicPlaneIn *string
	ConsumerGroup     *string
	ConsumerID        *string
	KafkaBrk          *string
	Oldest            *bool
	Verbose           *bool
	KafkaVersion      *string
	Assignor          *string
)

type KafkaProducerState struct {
	producer sarama.AsyncProducer
	topic    string
	key      *string
}

type Consumer struct {
	Ready chan bool
	Msgs  chan []byte
}

type KafkaConsumerState struct {
	Client   sarama.ConsumerGroup
	Consumer Consumer
	Topics   []string
}

// ParseKafkaVersion is a pass through to sarama.ParseKafkaVersion to get a KafkaVersion struct by a string version that can be passed into SetKafkaVersion
// This function is here so that calling code need not import sarama to set KafkaVersion

func RegisterFlags() {

	ConsumerID = flag.String("consumer-id", "", "App Consumer Id")
	KafkaTopicIn = flag.String("topic-in", "topicIn", "Kafka topic to consumer")
	KafkaTopicTable = flag.String("topic-table", "tags", "Kafka topic to consumer for enriched")
	KafkaTopicPlaneIn = flag.String("topic-plane-in", "planeIn", "Kafka input topic with the plans to be consumed")
	KafkaTopicOut = flag.String("topic-Out", "topicOut", "Kafka topic to producer")
	KafkaBrk = flag.String("kafka-brokers", "localhost:29092", "Kafka brokers list separated by commas")
	ConsumerGroup = flag.String("kafka-consumer-group", "test_cmx", "Kafka consumer group definition")
	Oldest = flag.Bool("kafka-oldest", false, "Kafka consumer consume initial offset from oldest")
	Verbose = flag.Bool("kafka-verbose", false, "Sarama logging")
	KafkaVersion = flag.String("kafka-version", "2.1.1", "Kafka cluster version")
	Assignor = flag.String("kafka-consumer-assignor", "range", "Consumer group partition assignment strategy (range, roundrobin, sticky)")
}

func StartKafkaProducerFromTopic(topic string, key *string) (*KafkaProducerState, error) {
	addrs := strings.Split(*KafkaBrk, ",")
	return StartKafkaProducer(addrs, topic, key)
}

func StartKafkaProducerFromArgs() (*KafkaProducerState, error) {
	addrs := strings.Split(*KafkaBrk, ",")
	return StartKafkaProducer(addrs, *KafkaTopicOut, nil)
}

func StartKafkaProducer(addrs []string, topic string, key *string) (*KafkaProducerState, error) {
	kafkaConfig := sarama.NewConfig()
	version, err := sarama.ParseKafkaVersion(*KafkaVersion)
	if err != nil {
		log.Panicf("Error parsing Kafka version: %v", err)
	}
	kafkaConfig.Version = version
	kafkaConfig.Producer.Return.Successes = false
	kafkaConfig.Producer.Partitioner = sarama.NewRoundRobinPartitioner

	kafkaProducer, err := sarama.NewAsyncProducer(addrs, kafkaConfig)
	if err != nil {
		return nil, err
	}

	realTopic := topic
	if *ConsumerID != "" {
		realTopic = fmt.Sprintf("%s_%s", *ConsumerID, topic)
	}
	state := KafkaProducerState{
		producer: kafkaProducer,
		topic:    realTopic,
		key:      key,
	}

	return &state, nil
}

func (s KafkaProducerState) SendKafkaDataMessage(msg []byte) {
	// log.Printf("Sending enriched message to topic %s\n", s.topic)
	var key string
	if s.key != nil {
		key, _ = jsonparser.GetString(msg, *s.key)
	}
	s.producer.Input() <- &sarama.ProducerMessage{
		Topic: s.topic,
		Value: sarama.ByteEncoder(msg),
		Key:   sarama.StringEncoder(key),
	}
}

func (s KafkaProducerState) Publish(msg []byte) {
	// fmt.Println(s.key)
	// fmt.Println(s.topic)
	// fmt.Println(s.GetTopic())
	// Inc metric
	utils.MetricMsgsOut.With(
		prometheus.Labels{
			"topic":    s.topic,
			"msg_type": "type msg",
		}).Inc()
	s.SendKafkaDataMessage(msg)
}

func (s KafkaProducerState) Close() error {
	err := s.producer.Close()
	return err
}

func (s KafkaProducerState) GetTopic() string {
	return s.topic
}

// Consumer

func StartConsumerFromTopic(topics []string, consumerGroup string, offset string) (*KafkaConsumerState, error) {
	t := topics
	if *ConsumerID != "" {
		for i := 0; i < len(t); i++ {
			t[i] = fmt.Sprintf("%s_%s", *ConsumerID, t[i])
		}
	}

	var oldest bool
	if offset == "latest" {
		oldest = false
	} else {
		oldest = true
	}
	return StartKafkaConsumer(strings.Split(*KafkaBrk, ","), consumerGroup,
		t, *Verbose, *Assignor, oldest)
}

func StartKafkaConsumerFromArgs() (*KafkaConsumerState, error) {
	realTopics := strings.Split(*KafkaTopicIn, ",")
	if *ConsumerID != "" {
		for i, topic := range realTopics {
			realTopic := fmt.Sprintf("%s_%s", *ConsumerID, topic)
			realTopics[i] = realTopic
		}
	}

	return StartKafkaConsumer(strings.Split(*KafkaBrk, ","), *ConsumerGroup,
		realTopics, *Verbose, *Assignor, *Oldest)
}

func StartKafkaConsumer(brokers []string, group string, topics []string, verbose bool,
	assignor string, oldest bool) (*KafkaConsumerState, error) {
	if verbose {
		sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	}

	version, err := sarama.ParseKafkaVersion(*KafkaVersion)
	if err != nil {
		return nil, err
	}

	/**
	 * Construct a new Sarama configuration.
	 * The Kafka cluster version has to be defined before the consumer/producer is initialized.
	 */
	config := sarama.NewConfig()
	config.Version = version
	// config.Consumer.Group.Session.Timeout = 180 * time.Second

	switch assignor {
	case "sticky":
		config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
	case "roundrobin":
		config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	case "range":
		config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	default:
		log.Panicf("Unrecognized consumer group partition assignor: %s", assignor)
	}

	if oldest {
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	/**
	 * Setup a new Sarama consumer group
	 */
	consumer := Consumer{
		Ready: make(chan bool),
		Msgs:  make(chan []byte),
	}
	client, err := sarama.NewConsumerGroup(brokers, group, config)
	if err != nil {
		return nil, err
	}

	state := KafkaConsumerState{
		Client:   client,
		Consumer: consumer,
		Topics:   topics,
	}

	return &state, nil
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *Consumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(consumer.Ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		// log.Debugf("Message claimed: value = %s, timestamp = %v, topic = %s", string(message.Value), message.Timestamp, message.Topic)
		consumer.Msgs <- message.Value
		session.MarkMessage(message, "")
		// Update metrics
		utils.MetricMsgsIn.With(
			prometheus.Labels{
				"topic": message.Topic,
			}).Inc()

	}

	return nil
}

func (s KafkaConsumerState) Close() error {
	err := s.Client.Close()
	return err
}
