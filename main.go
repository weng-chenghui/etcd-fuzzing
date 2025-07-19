package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	episodes     int
	horizon      int
	savePath     string
	replicas     int
	requests     int
	numRuns      int
	recordTraces bool
	host         string
	port         int
	address      string
)

func main() {
	rootCommand := &cobra.Command{}
	rootCommand.PersistentFlags().IntVarP(&episodes, "episodes", "e", 10000, "Number of episodes to run")
	rootCommand.PersistentFlags().IntVar(&horizon, "horizon", 50, "Horizon of each episode")
	rootCommand.PersistentFlags().StringVarP(&savePath, "save", "s", "results", "Save the results to the specified path")
	rootCommand.PersistentFlags().IntVarP(&replicas, "replicas", "r", 3, "Num of replicas to run in environment")
	rootCommand.PersistentFlags().IntVar(&requests, "requests", 1, "Num of initial requests to serve")
	rootCommand.PersistentFlags().IntVar(&numRuns, "runs", 5, "Number of runs to average over")
	rootCommand.PersistentFlags().BoolVar(&recordTraces, "record-traces", false, "Record the traces explored")
	rootCommand.PersistentFlags().StringVar(&host, "host", "127.0.0.1", "Host address to use")
	rootCommand.PersistentFlags().IntVar(&port, "port", 2023, "Port to use")
	rootCommand.AddCommand(FuzzCommand())
	rootCommand.AddCommand(OneCommand())

	if err := rootCommand.Execute(); err != nil {
		fmt.Println(err)
	}
}

func FuzzCommand() *cobra.Command {
	return &cobra.Command{
		Use: "fuzz",
		RunE: func(cmd *cobra.Command, args []string) error {
			address = fmt.Sprintf("%s:%d", host, port)
			fuzzer := NewFuzzer(&FuzzerConfig{
				Iterations: episodes,
				Steps:      horizon,
				Strategy:   NewRandomStrategy(),
				Guider:     NewLineCoverageGuider(address, "traces", recordTraces),
				Mutator:    &EmptyMutator{},
				RaftEnvironmentConfig: RaftEnvironmentConfig{
					Replicas:      replicas,
					ElectionTick:  20,
					HeartbeatTick: 2,
					TicksPerStep:  2,
				},
				MutPerTrace:        5,
				NumberRequests:     requests,
				CrashQuota:         2,
				MaxMessages:        10,
				SeedPopulationSize: 10,
				// Must specify a value otherwise it throws the div zero error.
				ReseedFrequency: 2000,
			})
			fuzzer.Run()
			return nil
		},
	}
}

func OneCommand() *cobra.Command {
	return &cobra.Command{
		Use: "compare",
		Run: func(cmd *cobra.Command, args []string) {
			address = fmt.Sprintf("%s:%d", host, port)
			c := NewComparision(savePath, &FuzzerConfig{
				Iterations: episodes,
				Steps:      horizon,
				Strategy:   NewRandomStrategy(),
				Mutator:    &EmptyMutator{},
				Checker:    SerializabilityChecker(),
				RaftEnvironmentConfig: RaftEnvironmentConfig{
					Replicas: replicas,
					// Higher election tick gives random better chances. (less timeouts)
					ElectionTick:  12,
					HeartbeatTick: 2,
					// Should not be more than ElectionTick/4 otherwise you are more likely to starve processes
					TicksPerStep: 3,
				},
				// Too much is bad, can lead to very local search
				MutPerTrace:    5,
				NumberRequests: requests,
				// More makes random worse
				CrashQuota: 10,
				// Too few messages are better for random
				MaxMessages:        5,
				SeedPopulationSize: 10,
				ReseedFrequency:    2000,
			}, numRuns)
			combinedMutator := CombineMutators(NewSwapCrashNodeMutator(2), NewSwapNodeMutator(20), NewSwapMaxMessagesMutator(20))
			c.Add("traceCov", combinedMutator, NewTraceCoverageGuider(address, "traces", recordTraces))
			c.Add("lineCov", combinedMutator, NewLineCoverageGuider(address, "traces", recordTraces))
			c.Add("tlcstate", combinedMutator, NewTLCStateGuider(address, "traces", recordTraces))
			c.Add("random", &EmptyMutator{}, NewTLCStateGuider(address, "traces", recordTraces))

			c.Run()
		},
	}
}
