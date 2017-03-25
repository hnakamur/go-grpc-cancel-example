package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"

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

func (c *exampleClient) runJob(ctx context.Context) error {
	stream, err := c.client.RunJob(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	err = stream.CloseSend()
	if err != nil {
		return errors.WithStack(err)
	}
	for {
		result, err := stream.Recv()
		if err == io.EOF {
			log.Printf("got EOF")
			break
		} else if grpc.Code(err) == codes.Canceled {
			log.Printf("canceled")
			break
		} else if err != nil {
			return errors.WithStack(err)
		}

		log.Printf("result=%+v", result)
	}
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
		log.Printf("failed to runJob err (%T)=%+v", errors.Cause(err), err)
	}
}
