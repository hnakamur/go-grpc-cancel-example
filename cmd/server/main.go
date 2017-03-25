package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/pkg/errors"

	pb "github.com/hnakamur/go-grpc-cancel-example"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
)

type exampleServer struct {
}

func newExampleServer() *exampleServer {
	return new(exampleServer)
}

func isEOFOrCanceled(err error) bool {
	return err == io.EOF || grpc.Code(err) == codes.Canceled
}

func (s *exampleServer) RunJob(stream pb.ExampleService_RunJobServer) error {
	log.Print("entered RunJob")
	defer log.Print("exiting from RunJob")
	go func() {
		defer log.Printf("exiting goroutine in RunJob")
		for {
			msg, err := stream.Recv()
			if isEOFOrCanceled(err) {
				log.Printf("got EOF or canceled")
				break
			} else if err != nil {
				log.Printf("failed to Recv msg; err (%T)=%+v", err, errors.WithStack(err))
				break
			}
			log.Printf("msg=%+v", msg)
			if msg.Type == "cancel" {
				log.Printf("received cancel JobControl")
				break
			}
		}
	}()

	ctx := stream.Context()
	rc := 0
	tick := time.Tick(time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-ctx.Done():
			log.Printf("received from ctx.Done #1")
			return nil
		default:
			res := &pb.JobResult{
				Type: "stdout",
				Data: fmt.Sprintf("%d", i),
			}
			err := stream.Send(res)
			if err != nil {
				err = errors.WithStack(err)
				log.Printf("failed to send stdout; err=%+v", err)
				return err
			}
		}

		select {
		case <-ctx.Done():
			log.Printf("received from ctx.Done #2")
			return nil
		case <-tick:
			// do nothing but just wait
		}
	}
	select {
	case <-ctx.Done():
		log.Printf("received from ctx.Done #3")
		return nil
	default:
		res := &pb.JobResult{
			Type: "rc",
			Data: fmt.Sprintf("%d", rc),
		}
		err := stream.Send(res)
		if err != nil {
			err = errors.WithStack(err)
			log.Printf("failed to send rc; err=%+v", err)
			return err
		}
		log.Printf("sent JobResult res=%+v", res)
	}
	return nil
}

func main() {
	var addr string
	flag.StringVar(&addr, "addr", ":10000", "server listen address")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		grpclog.Fatal(err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterExampleServiceServer(grpcServer, newExampleServer())
	grpcServer.Serve(lis)
}
