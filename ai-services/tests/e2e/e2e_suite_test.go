package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/project-ai-services/ai-services/tests/e2e/bootstrap"
	"github.com/project-ai-services/ai-services/tests/e2e/cleanup"
	"github.com/project-ai-services/ai-services/tests/e2e/cli"
	"github.com/project-ai-services/ai-services/tests/e2e/config"
	"github.com/project-ai-services/ai-services/tests/e2e/ingestion"
	"github.com/project-ai-services/ai-services/tests/e2e/podman"
	"github.com/project-ai-services/ai-services/tests/e2e/rag"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var (
	cfg                *config.Config
	runID              string
	appName            string
	tempDir            string
	tempBinDir         string
	aiServiceBin       string
	binVersion         string
	ctx                context.Context
	podmanReady        bool
	templateName       string
	goldenPath         string
	ragBaseURL         string
	judgeBaseURL       string
	mainPodsByTemplate map[string][]string
)

const (
	defaultAccuracyThreshold = 0.70
	defaultMaxRetries        = 2
)

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "AI Services E2E Suite")
}

var _ = ginkgo.BeforeSuite(func() {
	fmt.Println("[SETUP] Starting AI Services E2E setup")

	ctx = context.Background()

	ginkgo.By("Loading E2E configuration")
	cfg = &config.Config{}

	ginkgo.By("Generating unique run ID")
	runID = fmt.Sprintf("%d", time.Now().Unix())

	ginkgo.By("Preparing runtime environment")
	tempDir = bootstrap.PrepareRuntime(runID)
	gomega.Expect(tempDir).NotTo(gomega.BeEmpty())

	ginkgo.By("Preparing temp bin directory for test binaries")
	tempBinDir = fmt.Sprintf("%s/bin", tempDir)
	bootstrap.SetTestBinDir(tempBinDir)
	fmt.Printf("[SETUP] Test binary directory: %s\n", tempBinDir)

	ginkgo.By("Setting template name")
	templateName = "rag"

	ginkgo.By("Setting application name")
	appName = fmt.Sprintf("%s-app-%s", templateName, runID)

	ginkgo.By("Setting main pods by template")
	mainPodsByTemplate = map[string][]string{
		"rag": {
			"vllm-server",
			"milvus",
			"chat-bot",
		},
	}

	ginkgo.By("Setting golden dataset path")
	_, filename, _, _ := runtime.Caller(0)                        // returns the file path of this test file (e2e_suite_test.go)
	e2eDir := filepath.Dir(filename)                              // resolves ai-services/tests/e2e
	repoRoot := filepath.Clean(filepath.Join(e2eDir, "../../..")) // navigates to the workspace root
	goldenPath = filepath.Join(
		repoRoot,
		"test",
		"golden",
		"golden.csv",
	)

	ginkgo.By("Setting up LLM-as-Judge")
	if err := rag.SetupLLMAsJudge(ctx, cfg, runID); err != nil {
		ginkgo.Fail(fmt.Sprintf("failed to setup LLM-as-Judge: %v", err))
	}

	ginkgo.By("Building or verifying ai-services CLI")
	var err error
	aiServiceBin, err = bootstrap.BuildOrVerifyCLIBinary(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(aiServiceBin).NotTo(gomega.BeEmpty())
	cfg.AIServiceBin = aiServiceBin

	ginkgo.By("Getting ai-services version")
	binVersion, err = bootstrap.CheckBinaryVersion(aiServiceBin)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	fmt.Printf("[SETUP] ai-services version: %s\n", binVersion)

	ginkgo.By("Checking Podman environment (non-blocking)")
	err = bootstrap.CheckPodman()
	if err != nil {
		podmanReady = false
		fmt.Printf("[SETUP] [WARNING] Podman not available: %v - will be installed via bootstrap configure\n", err)
	} else {
		podmanReady = true
		fmt.Printf("[SETUP] Podman environment verified\n")
	}

	fmt.Printf("[SETUP] ================================================\n")
	fmt.Printf("[SETUP] E2E Environment Ready\n")
	fmt.Printf("[SETUP] Binary:   %s\n", aiServiceBin)
	fmt.Printf("[SETUP] Version:  %s\n", binVersion)
	fmt.Printf("[SETUP] TempDir:  %s\n", tempDir)
	fmt.Printf("[SETUP] RunID:    %s\n", runID)
	fmt.Printf("[SETUP] Podman:   %v\n", podmanReady)
	fmt.Printf("[SETUP] ================================================\n\n")
})

// Teardown after all tests have run.
var _ = ginkgo.AfterSuite(func() {
	fmt.Println("[TEARDOWN] AI Services E2E teardown")
	ginkgo.By("Cleaning up LLM-as-Judge container")
	if err := rag.CleanupLLMAsJudge(runID); err != nil {
		fmt.Printf("[TEARDOWN] Judge cleanup failed: %v\n", err)
	}
	ginkgo.By("Cleaning up E2E environment")
	if err := cleanup.CleanupTemp(tempDir); err != nil {
		fmt.Printf("[TEARDOWN] cleanup failed: %v\n", err)
	}
	ginkgo.By("Cleanup completed")
})

var _ = ginkgo.Describe("AI Services End-to-End Tests", ginkgo.Ordered, func() {
	ginkgo.Context("Help Command Tests", func() {
		ginkgo.It("runs help command", ginkgo.Label("spyre-independent"), func() {
			args := []string{"help"}
			output, err := cli.HelpCommand(ctx, cfg, args)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cli.ValidateHelpCommandOutput(output)).To(gomega.Succeed())
		})
		ginkgo.It("runs -h command", ginkgo.Label("spyre-independent"), func() {
			args := []string{"-h"}
			output, err := cli.HelpCommand(ctx, cfg, args)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cli.ValidateHelpCommandOutput(output)).To(gomega.Succeed())
		})
		ginkgo.It("runs help for a given random command", ginkgo.Label("spyre-independent"), func() {
			possibleCommands := []string{"application", "bootstrap", "completion", "version"}
			randomIndex := rand.Intn(len(possibleCommands))
			randomCommand := possibleCommands[randomIndex]
			args := []string{randomCommand, "-h"}
			output, err := cli.HelpCommand(ctx, cfg, args)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cli.ValidateHelpRandomCommandOutput(randomCommand, output)).To(gomega.Succeed())
		})
	})
	ginkgo.Context("Application Template Command Tests", ginkgo.Label("spyre-independent"), func() {
		ginkgo.It("runs application template command", func() {
			output, err := cli.TemplatesCommand(ctx, cfg)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cli.ValidateApplicationsTemplateCommandOutput(output)).To(gomega.Succeed())
		})
	})
	ginkgo.Context("Application Model Command Tests", ginkgo.Label("spyre-independent"), func() {
		ginkgo.It("verifies application model list command", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			output, err := cli.ModelList(ctx, cfg, templateName)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cli.ValidateModelListOutput(output, templateName)).To(gomega.Succeed())
			fmt.Printf("[TEST] Application model list validated successfully!\n")
		})
		ginkgo.It("verifies application model info command", ginkgo.Label("spyre-independent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			output, err := cli.ModelDownload(ctx, cfg, templateName)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cli.ValidateModelDownloadOutput(output, templateName)).To(gomega.Succeed())
			fmt.Printf("[TEST] Application model download validated successfully!\n")
		})
	})
	ginkgo.Context("Bootstrap Steps", func() {
		ginkgo.It("runs bootstrap configure", ginkgo.Label("spyre-dependent"), func() {
			output, err := cli.BootstrapConfigure(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cli.ValidateBootstrapConfigureOutput(output)).To(gomega.Succeed())
		})
		ginkgo.It("runs bootstrap validate", ginkgo.Label("spyre-dependent"), func() {
			output, err := cli.BootstrapValidate(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cli.ValidateBootstrapValidateOutput(output)).To(gomega.Succeed())
		})
		ginkgo.It("runs full bootstrap", ginkgo.Label("spyre-dependent"), func() {
			output, err := cli.Bootstrap(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cli.ValidateBootstrapFullOutput(output)).To(gomega.Succeed())
		})
	})
	ginkgo.Context("Application Image Command Tests", func() {
		ginkgo.It("lists images for rag template", ginkgo.Label("spyre-independent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			err := cli.ListImage(ctx, cfg, templateName)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			fmt.Printf("[TEST] Images listed successfully for %s template\n", templateName)
		})
		ginkgo.It("pulls images for rag template", ginkgo.Label("spyre-independent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			err := cli.PullImage(ctx, cfg, templateName)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			fmt.Printf("[TEST] Images pulled successfully for %s template\n", templateName)
		})
	})
	ginkgo.Context("Application Lifecycle", func() {
		ginkgo.It("creates rag application, runs health checks and validates RAG endpoints", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
			defer cancel()

			appName = fmt.Sprintf("rag-app-%s", runID)
			pods := []string{"backend", "ui", "db"} // replace with actual pod names

			err := cli.CreateRAGAppAndValidate(
				ctx,
				cfg,
				appName,
				"rag",
				"ui.port=3100,backend.port=5100",
				"5100", // backend port
				"3100", //ui port
				cli.CreateOptions{
					SkipModelDownload: false,
					ImagePullPolicy:   "IfNotPresent",
				},
				pods,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ragBaseURL, err = cli.GetBaseURL(createOutput, "5000")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			judgePort := os.Getenv("LLM_JUDGE_PORT")
			if judgePort == "" {
				judgePort = "8011"
			}

			judgeBaseURL, err = cli.GetBaseURL(createOutput, judgePort)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			fmt.Printf("[TEST] Application %s created, healthy, and RAG endpoints validated\n", appName)
		})
	})
	ginkgo.Context("Application Observability", func() {
		ginkgo.It("verifies application ps output", ginkgo.Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			cases := map[string][]string{
				"normal": nil,
				"wide":   {"-o", "wide"},
			}

			for name, flags := range cases {
				ginkgo.By(fmt.Sprintf("running application ps %s", name))

				output, err := cli.ApplicationPS(ctx, cfg, appName, flags...)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cli.ValidateApplicationPS(output)).To(gomega.Succeed())
			}
		})
		ginkgo.It("verifies application info output", ginkgo.Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			infoOutput, err := cli.ApplicationInfo(ctx, cfg, appName)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(cli.ValidateApplicationInfo(infoOutput, appName, templateName)).To(gomega.Succeed())
			fmt.Printf("[TEST] Application info output validated successfully!\n")
		})
		ginkgo.It("Verifies pods existence, health status  and restart count", ginkgo.Label("spyre-dependent"), func() {
			if !podmanReady {
				ginkgo.Skip("Podman not available - will be installed via bootstrap configure")
			}
			err := podman.VerifyContainers(appName)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "verify containers failed")
			fmt.Println("[TEST] Containers verified")
		})
		ginkgo.It("Verifies Exposed Ports of the application", ginkgo.Label("spyre-dependent"), func() {
			if !podmanReady {
				ginkgo.Skip("Podman not available - will be installed via bootstrap configure")
			}
			err := podman.VerifyExposedPorts(appName)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Verify exposed ports failed")
			fmt.Println("[TEST] Exposed ports verified")
		})
	})
	ginkgo.Context("Application Teardown", func() {
		ginkgo.It("stops the application", ginkgo.Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			suffixes, ok := mainPodsByTemplate[templateName]
			gomega.Expect(ok).To(gomega.BeTrue(), "unknown templateName")

			pods := make([]string, 0, len(suffixes))
			for _, s := range suffixes {
				pods = append(pods, fmt.Sprintf("%s--%s", appName, s))
			}

			output, err := cli.StopAppWithPods(ctx, cfg, appName, pods)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(output).NotTo(gomega.BeEmpty())

			fmt.Printf("[TEST] Application %s stopped successfully using --pod\n", appName)
		})
		ginkgo.It("starts application pods", ginkgo.Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			output, err := cli.StartApplication(
				ctx,
				cfg,
				appName,
				cli.StartOptions{
					SkipLogs: false,
				},
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(output).NotTo(gomega.BeEmpty())
			fmt.Printf("[TEST] Application %s started successfully\n", appName)
		})
		ginkgo.It("starts document ingestion pod and validates ingestion completion", ginkgo.Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
			defer cancel()

			gomega.Expect(appName).NotTo(gomega.BeEmpty())

			gomega.Expect(ingestion.PrepareDocs(appName)).To(gomega.Succeed())

			gomega.Expect(ingestion.StartIngestion(ctx, cfg, appName)).To(gomega.Succeed())

			logs, err := ingestion.WaitForIngestionLogs(ctx, cfg, appName)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(logs).To(gomega.ContainSubstring("Ingestion started"))
			gomega.Expect(logs).To(gomega.ContainSubstring("Processed '/var/docs/test_doc.pdf'"))

			fmt.Printf("[TEST] Ingestion completed successfully for application %s\n", appName)
		})
		ginkgo.Context("RAG Golden Dataset Validation", func() {
			ginkgo.It("validates RAG answers against golden dataset", ginkgo.Label("spyre-dependent"), func() {
				fmt.Println("[RAG] Starting golden dataset validation")

				cases, err := rag.LoadGoldenCSV(goldenPath)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cases).NotTo(gomega.BeEmpty())

				accuracyThreshold := defaultAccuracyThreshold
				if v, err := strconv.ParseFloat(os.Getenv("RAG_ACCURACY_THRESHOLD"), 64); err == nil {
					accuracyThreshold = v
				}

				results := make([]rag.EvalResult, 0, len(cases))
				passed := 0
				total := len(cases)

				for i, tc := range cases {
					ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
					defer cancel()

					result := rag.EvalResult{
						Question: tc.Question,
						Passed:   false,
					}

					// 1. Ask RAG
					ragAns, ragErr := rag.RunWithRetry(ctx, defaultMaxRetries, func(ctx context.Context) (string, error) {
						return rag.AskRAG(ctx, ragBaseURL, tc.Question)
					})

					if ragErr != nil {
						result.Details = fmt.Sprintf("RAG request failed: %v", ragErr)
						results = append(results, result)

						continue
					}

					// 2. Ask Judge with format retry
					verdict, reason, err := rag.AskJudgeWithFormatRetry(
						ctx,
						defaultMaxRetries,
						judgeBaseURL,
						tc.Question,
						ragAns,
						tc.GoldenAnswer,
					)
					if err != nil {
						result.Details = fmt.Sprintf("Judge failed: %v", err)
						results = append(results, result)

						continue
					}

					result.Passed = verdict == "YES"
					result.Details = reason

					if result.Passed {
						passed++
					}

					results = append(results, result)
					fmt.Printf("[RAG] Evaluated question %d/%d | verdict=%s | reason=%s\n", i+1, total, verdict, reason)
				}

				accuracy := float64(passed) / float64(total)
				rag.PrintValidationSummary(results, accuracy)

				if accuracy < accuracyThreshold {
					ginkgo.Fail(fmt.Sprintf(
						"RAG accuracy %.2f below threshold %.2f",
						accuracy,
						accuracyThreshold,
					))
				}

				fmt.Println("[RAG] Golden dataset validation completed")
			})
		})
		ginkgo.It("deletes the application using --skip-cleanup", ginkgo.Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			output, err := cli.DeleteAppSkipCleanup(ctx, cfg, appName)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(output).NotTo(gomega.BeEmpty())

			fmt.Printf("[TEST] Application %s deleted successfully using --skip-cleanup\n", appName)
		})
	})
})
