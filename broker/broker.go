// Copyright 2021-2022 The Memphis Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package broker

import (
	"errors"
	"memphis-control-plane/config"
	"memphis-control-plane/logger"
	"memphis-control-plane/models"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

var configuration = config.GetConfig()
var connectionChannel = make(chan bool)
var connected = false

func getErrorWithoutNats(err error) error {
	message := strings.ToLower(err.Error())
	message = strings.Replace(message, "nats", "memphis-broker", -1)
	return errors.New(message)
}

func handleDisconnectEvent(con *nats.Conn, err error) {
	logger.Error("Broker has disconnected: " + err.Error())
}

func handleAsyncErrors(con *nats.Conn, sub *nats.Subscription, err error) {
	logger.Error("Broker has experienced an error: " + err.Error())
}

func handleReconnect(con *nats.Conn) {
	if connected {
		logger.Error("Reconnected to the broker")
	}
	connectionChannel <- true
}

func handleClosed(con *nats.Conn) {
	if !connected {
		logger.Info("All reconnect attempts with the broker were failed")
		connectionChannel <- false
	}
}

func sigHandler(nonce []byte, seed string) ([]byte, error) {
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return nil, err
	}

	defer kp.Wipe()

	sig, _ := kp.Sign(nonce)
	return sig, nil
}

func userCredentials(userJWT string, userKeySeed string) nats.Option {
	userCB := func() (string, error) {
		return userJWT, nil
	}
	sigCB := func(nonce []byte) ([]byte, error) {
		return sigHandler(nonce, userKeySeed)
	}
	return nats.UserJWT(userCB, sigCB)
}

func initializeBrokerConnection() (*nats.Conn, nats.JetStreamContext) {
	nc, err := nats.Connect(
		configuration.BROKER_URL,
		// nats.UserCredentials("admin3.creds"),
		// userCredentials(configuration.BROKER_ADMIN_JWT, configuration.BROKER_ADMIN_NKEY),
		nats.Token(configuration.CONNECTION_TOKEN),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(5*time.Second),
		nats.Timeout(10*time.Second),
		nats.PingInterval(5*time.Second),
		nats.DisconnectErrHandler(handleDisconnectEvent),
		nats.ErrorHandler(handleAsyncErrors),
		nats.ReconnectHandler(handleReconnect),
		nats.ClosedHandler(handleClosed),
	)

	if !nc.IsConnected() {
		isConnected := <-connectionChannel
		if !isConnected {
			logger.Error("Failed to create connection with the broker")
			panic("Failed to create connection with the broker")
		}
	}

	if err != nil {
		logger.Error("Failed to create connection with the broker: " + err.Error())
		panic("Failed to create connection with the broker: " + err.Error())
	}

	js, err := nc.JetStream()
	if err != nil {
		logger.Error("Failed to create connection with the broker: " + err.Error())
		panic("Failed to create connection with the broker: " + err.Error())
	}

	connected = true
	logger.Info("Established connection with the broker")
	return nc, js
}

func AddUser(username string) (string, error) {
	return configuration.CONNECTION_TOKEN, nil
}

func RemoveUser(username string) error {
	return nil
}

func CreateStream(station models.Station) error {
	var maxMsgs int
	if station.RetentionType == "messages" && station.RetentionValue > 0 {
		maxMsgs = station.RetentionValue
	} else {
		maxMsgs = -1
	}

	var maxBytes int
	if station.RetentionType == "bytes" && station.RetentionValue > 0 {
		maxBytes = station.RetentionValue
	} else {
		maxBytes = -1
	}

	var maxAge time.Duration
	if station.RetentionType == "message_age_sec" && station.RetentionValue > 0 {
		maxAge = time.Duration(station.RetentionValue) * time.Second
	} else {
		maxAge = time.Duration(0)
	}

	var storage nats.StorageType
	if station.StorageType == "memory" {
		storage = nats.MemoryStorage
	} else {
		storage = nats.FileStorage
	}

	var dedupWindow time.Duration
	if station.DedupEnabled {
		dedupWindow = time.Duration(station.DedupWindowInMs*1000) * time.Nanosecond
	} else {
		dedupWindow = time.Duration(1) * time.Nanosecond // can not be 0
	}

	_, err := js.AddStream(&nats.StreamConfig{
		Name:              station.Name,
		Subjects:          []string{station.Name + ".*"},
		Retention:         nats.LimitsPolicy,
		MaxConsumers:      -1,
		MaxMsgs:           int64(maxMsgs),
		MaxBytes:          int64(maxBytes),
		Discard:           nats.DiscardOld,
		MaxAge:            maxAge,
		MaxMsgsPerSubject: -1,
		MaxMsgSize:        int32(configuration.MAX_MESSAGE_SIZE_MB) * 1024,
		Storage:           storage,
		Replicas:          station.Replicas,
		NoAck:             false,
		Duplicates:        dedupWindow,
	}, nats.MaxWait(15*time.Second))
	if err != nil {
		return getErrorWithoutNats(err)
	}

	return nil
}

func CreateProducer() error {
	// nothing to create
	return nil
}

func CreateConsumer(consumer models.Consumer, station models.Station) error {
	var consumerName string
	if consumer.ConsumersGroup != "" {
		consumerName = consumer.ConsumersGroup
	} else {
		consumerName = consumer.Name
	}

	var maxAckTimeMs int64
	if consumer.MaxAckTimeMs <= 0 {
		maxAckTimeMs = 30000 // 30 sec
	}

	_, err := js.AddConsumer(station.Name, &nats.ConsumerConfig{
		Durable:       consumerName,
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       time.Duration(maxAckTimeMs) * time.Millisecond,
		MaxDeliver:    10,
		FilterSubject: station.Name + ".final",
		ReplayPolicy:  nats.ReplayInstantPolicy,
		MaxAckPending: -1,
		HeadersOnly:   false,
		// RateLimit: ,// Bits per sec
		// Heartbeat: // time.Duration,
	})
	if err != nil {
		return getErrorWithoutNats(err)
	}

	return nil
}

func RemoveStream(streamName string) error {
	err := js.DeleteStream(streamName)
	if err != nil {
		return getErrorWithoutNats(err)
	}

	return nil
}

func RemoveProducer() error {
	// nothing to remove
	return nil
}

func RemoveConsumer(streamName string, consumerName string) error {
	err := js.DeleteConsumer(streamName, consumerName)
	if err != nil {
		return getErrorWithoutNats(err)
	}

	return nil
}

func ValidateUserCreds(token string) error {
	nc, err := nats.Connect(
		configuration.BROKER_URL,
		// nats.UserCredentials("admin3.creds"),
		// userCredentials(configuration.BROKER_ADMIN_JWT, configuration.BROKER_ADMIN_NKEY),
		nats.Token(token),
	)

	if err != nil {
		return getErrorWithoutNats(err)
	}

	_, err = nc.JetStream()
	if err != nil {
		return getErrorWithoutNats(err)
	}

	nc.Close()
	return nil
}

func Close() {
	broker.Close()
}

var broker, js = initializeBrokerConnection()