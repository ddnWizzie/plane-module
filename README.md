# Enriched Cmx Plane Engine
The application consumes a topic of kafka that contains the information of the plans with their zones. 
The name of the plan and its zones are stored in a structure, as well as the information of the layouts and their zones. We use sync.map

The objective is that given a point, the algorithm calculates if the point belongs to a plane and to which zones of the plane, and if it belongs to any layout and its zones.

Devuelve un json con el nombre del plano del layouts y las zonas que pertenece el punto.
The json is enriched with data from the social distancing algorithm, which makes a series of transformations with the latitude and longitude of the point.



## Requirements

- vsCode
- Languge go 
- kafka server running


## Install Dependencies
```sh
  go get -u github.com/Shopify/sarama
  go get -u github.com/sirupsen/logrus
  go get -u github.com/kellydunn/golang-geo
  go get -u github.com/satori/go.uuid
  go get -u github.com/buger/jsonparser
  go get -u github.com/prometheus/client_golang/prometheus
```

## Run App
```sh
  cd cmd
  go run main.go
```

## Environment Variables
| Name | Description |
| ------ | ------ |
| ConsumerID | App Consumer Id |
| KafkaTopicIn | Kafka topic to consumer |
| KafkaTopicPlaneIn | Kafka input topic with the plans |
| KafkaBrk |  Kafka brokers list separated by commas |