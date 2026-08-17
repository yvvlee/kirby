package kirby

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

const defaultHealthcheckTimeout = 3 * time.Second

type healthcheckOptions struct {
	httpEndpoint string
	grpcAddress  string
	timeout      time.Duration
}

func newHealthcheckCommand() *cobra.Command {
	options := healthcheckOptions{}
	command := &cobra.Command{
		Use:          "healthcheck",
		Short:        "Check the local HTTP and gRPC listeners",
		SilenceUsage: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return runHealthcheck(command.Context(), options)
		},
	}
	command.Flags().StringVar(&options.httpEndpoint, "http-endpoint", "http://127.0.0.1:8080/readyz", "HTTP readiness endpoint")
	command.Flags().StringVar(&options.grpcAddress, "grpc-address", "127.0.0.1:9090", "gRPC health service address")
	command.Flags().DurationVar(&options.timeout, "timeout", defaultHealthcheckTimeout, "total probe timeout")
	return command
}

func runHealthcheck(parent context.Context, options healthcheckOptions) error {
	if options.httpEndpoint == "" {
		return fmt.Errorf("HTTP readiness endpoint is required")
	}
	if options.grpcAddress == "" {
		return fmt.Errorf("gRPC health service address is required")
	}
	if options.timeout <= 0 {
		return fmt.Errorf("healthcheck timeout must be greater than zero")
	}

	ctx, cancel := context.WithTimeout(parent, options.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, options.httpEndpoint, nil)
	if err != nil {
		return fmt.Errorf("build HTTP readiness request: %w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("check HTTP readiness: %w", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("check HTTP readiness: unexpected status %s", response.Status)
	}

	connection, err := grpc.NewClient(options.grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create gRPC health client: %w", err)
	}
	defer connection.Close()
	reply, err := grpc_health_v1.NewHealthClient(connection).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("check gRPC health: %w", err)
	}
	if reply.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("check gRPC health: status is %s", reply.GetStatus())
	}
	return nil
}
