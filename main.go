package main

import (
	"context"
	"log"
	"os"

	"github.com/hashicorp/nomad/api"
)

func main() {
	me := os.Getenv("NOMAD_ALLOC_ID")

	nc, err := api.NewClient(&api.Config{
		Address:  os.Getenv("NOMAD_ADDR"),
		SecretID: os.Getenv("NOMAD_TOKEN"),
	})
	if err != nil {
		log.Fatal(err)
	}
	var index uint64 = 0
	if _, meta, err := nc.Jobs().List(nil); err == nil {
		index = meta.LastIndex
	}
	alloc, _, err := nc.Allocations().Info(me, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("I am running on: %v", alloc.NodeName)

	topics := map[api.Topic][]string{}
	_ = topics
	ctx := context.Background()
	eventsClient := nc.EventStream()
	eventCh, err := eventsClient.Stream(ctx, topics, index, &api.QueryOptions{})
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case <-ctx.Done():
			os.Exit(0)

		case event := <-eventCh:

			if event.IsHeartbeat() {
				continue
			}

			for _, task := range event.Events {
				if task.Type == "ServiceRegistration" || task.Type == "ServiceDeregistration" {
					serv, _ := task.Service()
					sl, _, _ := nc.Services().Get(serv.ServiceName, &api.QueryOptions{})
					for _, s := range sl {
						alloc, _, err := nc.Allocations().Info(s.AllocID, &api.QueryOptions{})
						if err != nil {
							log.Printf("%v", err)
							continue
						}
						if task.Type == "ServiceRegistration" {
							log.Printf("iptables-add %v %v %v", s.ServiceName, s.Address, alloc.NodeName)
						} else {
							log.Printf("iptables-remove %v %v %v", s.ServiceName, s.Address, alloc.NodeName)
						}
					}
				}
			}
		}
	}
}
