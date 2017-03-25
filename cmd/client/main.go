package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"

	pb "github.com/hnakamur/go-grpc-cancel-example"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type exampleClient struct {
	client pb.ExampleServiceClient
}

func newExampleClient(client pb.ExampleServiceClient) *exampleClient {
	return &exampleClient{
		client: client,
	}
}

func isEOFOrCanceled(err error) bool {
	return err == io.EOF || grpc.Code(err) == codes.Canceled
}

func (c *exampleClient) runJob(ctx context.Context) error {
	stream, err := c.client.RunJob(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			result, err := stream.Recv()
			if isEOFOrCanceled(err) {
				log.Printf("got EOF or canceled")
				cancel()
				return
			} else if err != nil {
				log.Printf("failed to Recv err=%+v", errors.WithStack(err))
				return
			}

			log.Printf("result=%+v", result)
		}
	}()

	<-ctx.Done()
	log.Printf("received from ctx.Done")
	wg.Wait()
	log.Printf("done wait")
	return nil
}

func main() {
	var serverAddr string
	flag.StringVar(&serverAddr, "server-addr", "127.0.0.1:10000", "server listen address")

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %+v", errors.WithStack(err))
	}
	defer conn.Close()

	client := newExampleClient(pb.NewExampleServiceClient(conn))

	log.Printf("Press ^C to interrupt")
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Printf("Got interrupted")
		cancel()
	}()
	err = client.runJob(ctx)
	if err != nil {
		log.Fatalf("failed to runJob; err=%+v", err)
	}
}
