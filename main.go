package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
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
	_ = index
	alloc, _, err := nc.Allocations().Info(me, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("I am running on: %v", alloc.NodeName)

	topics := map[api.Topic][]string{}
	_ = topics
	ctx := context.Background()
	eventsClient := nc.EventStream()
	eventCh, err := eventsClient.Stream(ctx, topics, 1<<63-1, &api.QueryOptions{})
	if err != nil {
		log.Fatal(err)
	}

	go func() {

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
	}()

	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
	if err != nil {
		panic(err)
	}

	ems, errs := cli.Events(context.Background(), types.EventsOptions{})
	go func() {
		for {
			select {
			case err := <-errs:
				fmt.Println(err)
			case em := <-ems:
				if em.Type == "network" && (em.Action == "connect" || em.Action == "disconnect") {
					info, err := cli.ContainerInspect(context.Background(), em.Actor.Attributes["container"])
					if err != nil {
						log.Printf("%v", err)
					}
					var allocID string
					var nodeName string
					for _, env := range info.Config.Env {
						if strings.Contains(env, "NOMAD_ALLOC_ID") {
							allocID = strings.Split(env, "=")[1]
							log.Printf("NOMAD_ALLOC_ID: %s", allocID)
						}
					}

					alloc, _, err := nc.Allocations().Info(allocID, nil)

					if err != nil {
						log.Printf("%v", err)
					}
					if err == nil {
						nodeName = alloc.NodeName
					}

					log.Printf(
						"Docker Event: Container %s %s to network %s ( alloc: %s) Node %s",
						info.Name,
						em.Action,
						info.NetworkSettings.Networks[em.Actor.Attributes["name"]].IPAddress,
						allocID,
						nodeName,
					)
				}
			}
		}
	}()
	select {}
}
