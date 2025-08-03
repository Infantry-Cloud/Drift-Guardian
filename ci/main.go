package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Payload represents the JSON structure expected by the environment endpoint
type Payload struct {
	RepoName        string `json:"repoName"`
	Branch          string `json:"branchName"`
	Environment     string `json:"environment"`
	EnvironmentTier string `json:"environmentTier"`
	DriftThreshold  string `json:"driftThreshold"`
	ProjectID       string `json:"projectId"`
	Operation       string `json:"operation"`
	ExitCode        int    `json:"exitCode"`
	Scheduled       bool   `json:"scheduled"`
	Timestamp       string `json:"timestamp"`            // Added to match server-side Payload
	PlanOutput      string `json:"planOutput,omitempty"` // Terraform plan output
}

// debugLog prints messages only when GUARDIAN_DEBUG is set to true
func debugLog(format string, args ...interface{}) {
	debugMode := false
	debugEnv := os.Getenv("GUARDIAN_DEBUG")
	if debugEnv != "" {
		parsedValue, err := strconv.ParseBool(debugEnv)
		if err == nil {
			debugMode = parsedValue
		}
	}

	if debugMode {
		fmt.Printf(format, args...)
	}
}

func main() {
	// Define command line flags for Drift Guardian configuration
	terraformPtr := flag.String("terraform-version", "", "The version of Terraform used for operations")
	endpointPtr := flag.String("drift-endpoint", "", "The URL of the Drift Guardian service (can also be set via DRIFT_GUARDIAN_ENDPOINT environment variable)")
	scheduledPtr := flag.Bool("drift-scheduled", false, "Whether this is a scheduled run (can also be set via SCHEDULED environment variable)")

	// Parse command line flags
	flag.Parse()

	// Get remaining arguments (these will be passed to terraform)
	tfArgs := flag.Args()

	// If no arguments provided, show usage
	if len(tfArgs) == 0 {
		debugLog("Usage: drift-guardian [drift-guardian flags] <terraform command> [terraform args]\n")
		debugLog("\nDrift Guardian flags:\n")
		flag.VisitAll(func(f *flag.Flag) {
			debugLog("  -%s: %s (default: %s)\n", f.Name, f.Usage, f.DefValue)
		})
		debugLog("\nAll other arguments are passed directly to terraform.\n")
		os.Exit(1)
	}

	// Extract the terraform operation (first argument)
	operation := tfArgs[0]

	// For 'plan' operation, ensure -detailed-exitcode is included
	if operation == "plan" {
		// Check if -detailed-exitcode is already in the arguments
		hasDetailedExitcode := false
		for _, arg := range tfArgs[1:] {
			if arg == "-detailed-exitcode" {
				hasDetailedExitcode = true
				break
			}
		}

		// If not present, add it
		if !hasDetailedExitcode {
			tfArgs = append(tfArgs, "-detailed-exitcode")
			debugLog("Added -detailed-exitcode flag to terraform plan command\n")
		}
	}

	// Check for endpoint in environment variable if not provided as flag
	endpoint := *endpointPtr
	if endpoint == "" {
		endpoint = os.Getenv("DRIFT_GUARDIAN_ENDPOINT")
	}

	terraformVersion := *terraformPtr
	if terraformVersion == "" {
		terraformVersion = os.Getenv("TERRAFORM_VERSION")
	}

	// Set TFENV_TERRAFORM_VERSION to the endpoint value
	_ = os.Setenv("TFENV_TERRAFORM_VERSION", terraformVersion)

	// Check if the scheduled flag was set in environment variable
	scheduled := *scheduledPtr
	if !scheduled {
		scheduledEnv := os.Getenv("SCHEDULED")
		if scheduledEnv != "" {
			parsedValue, err := strconv.ParseBool(scheduledEnv)
			if err == nil {
				scheduled = parsedValue
			}
		}
	}

	// Get GitLab environment variables
	projectID := os.Getenv("CI_PROJECT_ID")
	if projectID == "" {
		debugLog("Warning: CI_PROJECT_ID environment variable not set\n")
		projectID = "default"
	}

	repoName := os.Getenv("CI_PROJECT_NAME")
	if repoName == "" {
		// Fallback to CI_PROJECT_TITLE if CI_PROJECT_NAME is not available
		repoName = os.Getenv("CI_PROJECT_TITLE")
		if repoName == "" {
			debugLog("Warning: Neither CI_PROJECT_NAME nor CI_PROJECT_TITLE environment variables are set\n")
			repoName = "default"
		}
	}

	environment := os.Getenv("CI_ENVIRONMENT_NAME")
	if environment == "" {
		debugLog("Warning: CI_ENVIRONMENT_NAME environment variable not set, using 'default'\n")
		environment = "default"
	}

	environmentTier := os.Getenv("CI_ENVIRONMENT_TIER")
	if environmentTier == "" {
		debugLog("Warning: CI_ENVIRONMENT_TIER environment variable not set, using 'default'\n")
		environmentTier = "default"
	}

	driftThreshold := os.Getenv("DRIFT_THRESHOLD")
	if driftThreshold == "" {
		debugLog("Drift Threshold Override not setting, using 'default'\n")
	}

	branchName := os.Getenv("CI_COMMIT_BRANCH")
	if branchName == "" {
		debugLog("Warning: CI_COMMIT_BRANCH environment variable not set, using 'default'\n")
		branchName = "default"
	}

	// Log the configuration values
	debugLog("Drift Guardian CLI configured with:\n")
	debugLog("  Endpoint: %s\n", endpoint)
	debugLog("  Repository Name: %s\n", repoName)
	debugLog("  Project ID: %s\n", projectID)
	debugLog("  Branch Name: %s\n", branchName)
	debugLog("  Environment Tier: %s\n", environmentTier)
	debugLog("  Environment: %s\n", environment)
	debugLog("  Scheduled: %t\n", scheduled)
	debugLog("  Operation: %s\n", operation)
	debugLog("  Terraform Args: %v\n", tfArgs)

	// Get terraform binary path from environment variable or use default
	terraformBinary := os.Getenv("TERRAFORM_BINARY")
	if terraformBinary == "" {
		terraformBinary = "terraform"
	}

	// Create and execute the terraform command
	cmd := exec.Command(terraformBinary, tfArgs...)

	// Declare exitCode and err in the outer scope
	var exitCode int
	var err error

	// For plan operations, capture the output to include in the payload
	var planOutput string
	if operation == "plan" {
		// Create a buffer to capture the output
		var stdout, stderr bytes.Buffer
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
		cmd.Stdin = os.Stdin

		// Run the command
		debugLog("Executing: %s %s\n", terraformBinary, strings.Join(tfArgs, " "))
		err = cmd.Run()

		// Capture the combined output
		planOutput = stdout.String() + stderr.String() // Should add processing for the output

		// Determine the exit code
		exitCode = 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		// Log the exit code
		debugLog("Terraform command exited with code: %d\n", exitCode)
	} else {
		// For non-plan operations, just connect to parent process
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Run the terraform command
		debugLog("Executing: %s %s\n", terraformBinary, strings.Join(tfArgs, " "))
		err = cmd.Run()

		// Determine the exit code
		exitCode = 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		// Log the exit code
		debugLog("Terraform command exited with code: %d\n", exitCode)
	}

	// If endpoint is configured, send webhook to track drift
	if endpoint != "" {
		// Create payload
		payload := Payload{
			RepoName:        repoName,
			Branch:          branchName,
			Environment:     environment,
			EnvironmentTier: environmentTier,
			DriftThreshold:  driftThreshold,
			ProjectID:       projectID,
			Operation:       operation,
			ExitCode:        exitCode,
			Scheduled:       scheduled,
			Timestamp:       time.Now().Format(time.RFC3339),
		}

		// Add plan output for plan operations with drift detected
		if operation == "plan" && exitCode == 2 {
			// Limit the size of the plan output to avoid very large payloads
			const maxOutputSize = 50000 // 50KB limit
			if len(planOutput) > maxOutputSize {
				planOutput = planOutput[:maxOutputSize] + "\n... [output truncated due to size]\n"
			}
			payload.PlanOutput = planOutput
		}

		// Send webhook
		if operation == "plan" || operation == "apply" || operation == "destroy" {
			sendWebhook(endpoint, payload)
		}
	}

	// Exit with the same exit code as the terraform command
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			os.Exit(0)
		} else {
			// For non-ExitError errors, still exit with 1 as these are unexpected errors
			fmt.Printf("Error executing terraform: %v\n", err)
			os.Exit(1)
		}
	}
}
