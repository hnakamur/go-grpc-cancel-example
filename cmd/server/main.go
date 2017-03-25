package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"golang.org/x/sync/errgroup"

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

func (s *exampleServer) RunJob(stream pb.ExampleService_RunJobServer) error {
	defer log.Printf("exiting RunJob")
	var eg errgroup.Group
	eg.Go(func() error {
		defer log.Printf("exit receving goroutine")
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				log.Printf("got EOF")
				break
			} else if grpc.Code(err) == codes.Canceled {
				log.Printf("canceled")
				break
			} else if err != nil {
				return errors.WithStack(err)
			}
			log.Printf("msg=%+v", msg)
		}
		return nil
	})

	eg.Go(func() error {
		defer log.Printf("exit sending goroutine")
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
					return errors.WithStack(err)
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
				return errors.WithStack(err)
			}
		}
		return nil
	})

	err := eg.Wait()
	if err != nil {
		log.Printf("got error in RunJob; err (%T)=%+v", errors.Cause(err), err)
		return err
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
