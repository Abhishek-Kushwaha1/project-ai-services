package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
	"os"
	"strconv"
	"path/filepath"
	"runtime"

	"github.com/project-ai-services/ai-services/tests/e2e/bootstrap"
	"github.com/project-ai-services/ai-services/tests/e2e/cleanup"
	"github.com/project-ai-services/ai-services/tests/e2e/cli"
	"github.com/project-ai-services/ai-services/tests/e2e/config"
	"github.com/project-ai-services/ai-services/tests/e2e/ingestion"
	"github.com/project-ai-services/ai-services/tests/e2e/podman"
	"github.com/project-ai-services/ai-services/tests/e2e/rag"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	RegisterFailHandler(Fail)
	RunSpecs(t, "AI Services E2E Suite")
}

var _ = BeforeSuite(func() {
	fmt.Println("[SETUP] Starting AI Services E2E setup")

	ctx = context.Background()

	By("Loading E2E configuration")
	cfg = &config.Config{}

	By("Generating unique run ID")
	runID = fmt.Sprintf("%d", time.Now().Unix())

	By("Setting template name")
	templateName = "rag"

	By("Setting application name")
	appName = fmt.Sprintf("%s-app-%s", templateName, runID)

	By("Setting main pods by template")
	mainPodsByTemplate = map[string][]string{
		"rag": {
			"vllm-server",
			"milvus",
			"chat-bot",
		},
	}

	By("Setting golden dataset path")
	_, filename, _, _ := runtime.Caller(0) // returns the file path of this test file (e2e_suite_test.go)
	e2eDir := filepath.Dir(filename)       // resolves ai-services/tests/e2e
	repoRoot := filepath.Clean(filepath.Join(e2eDir, "../../..")) // navigates to the workspace root
	goldenPath = filepath.Join(
		repoRoot,
		"test",
		"golden",
		"golden.csv",
	)

	By("Setting up LLM-as-Judge")
	if err := rag.SetupLLMAsJudge(ctx, cfg, runID); err != nil {
		Fail(fmt.Sprintf("failed to setup LLM-as-Judge: %v", err))
	}

	By("Preparing runtime environment")
	tempDir = bootstrap.PrepareRuntime(runID)
	Expect(tempDir).NotTo(BeEmpty())

	By("Preparing temp bin directory for test binaries")
	tempBinDir = fmt.Sprintf("%s/bin", tempDir)
	bootstrap.SetTestBinDir(tempBinDir)
	fmt.Printf("[SETUP] Test binary directory: %s\n", tempBinDir)

	By("Building or verifying ai-services CLI")
	var err error
	aiServiceBin, err = bootstrap.BuildOrVerifyCLIBinary(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(aiServiceBin).NotTo(BeEmpty())
	cfg.AIServiceBin = aiServiceBin

	By("Getting ai-services version")
	binVersion, err = bootstrap.GetBinaryVersion(aiServiceBin)
	Expect(err).NotTo(HaveOccurred())
	fmt.Printf("[SETUP] ai-services version: %s\n", binVersion)

	By("Checking Podman environment (non-blocking)")
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

var _ = AfterSuite(func() {
	fmt.Println("[TEARDOWN] AI Services E2E teardown")

	By("Cleaning up LLM-as-Judge container")
	if err := rag.CleanupLLMAsJudge(runID); err != nil {
		fmt.Printf("[TEARDOWN] Judge cleanup failed: %v\n", err)
	}

	By("Cleaning up E2E environment")
	if err := cleanup.CleanupTemp(tempDir); err != nil {
		fmt.Printf("[TEARDOWN] cleanup failed: %v\n", err)
	}
	By("Cleanup completed")
})

var _ = Describe("AI Services End-to-End Tests", Ordered, func() {
	Context("Version Command Tests", func() {
		It("runs application version command", Label("spyre-independent"), func() {
			args := []string{"version"}
			output, err := cli.VersionCommand(ctx, cfg, args)
			voutput, coutput, gerr := cli.GitVersionCommands(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(gerr).NotTo(HaveOccurred())
			Expect(cli.ValidateVersionCommandOutput(output, voutput, coutput)).To(Succeed())
		})
	})
	Context("Help Command Tests", func() {
		It("runs help command", Label("spyre-independent"), func() {
			args := []string{"help"}
			output, err := cli.HelpCommand(ctx, cfg, args)
			Expect(err).NotTo(HaveOccurred())
			Expect(cli.ValidateHelpCommandOutput(output)).To(Succeed())
		})
		It("runs -h command", Label("spyre-independent"), func() {
			args := []string{"-h"}
			output, err := cli.HelpCommand(ctx, cfg, args)
			Expect(err).NotTo(HaveOccurred())
			Expect(cli.ValidateHelpCommandOutput(output)).To(Succeed())
		})
		It("runs help for a given random command", Label("spyre-independent"), func() {
			possibleCommands := []string{"application", "bootstrap", "completion", "version"}
			randomIndex := rand.Intn(len(possibleCommands))
			randomCommand := possibleCommands[randomIndex]
			args := []string{randomCommand, "-h"}
			output, err := cli.HelpCommand(ctx, cfg, args)
			Expect(err).NotTo(HaveOccurred())
			Expect(cli.ValidateHelpRandomCommandOutput(randomCommand, output)).To(Succeed())
		})
	})
	Context("Application Template Command Tests", Label("spyre-independent"), func() {
		It("runs application template command", func() {
			output, err := cli.TemplatesCommand(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(cli.ValidateApplicationsTemplateCommandOutput(output)).To(Succeed())
		})
	})
	Context("Application Model Command Tests", Label("spyre-independent"), func() {
		It("verifies application model list command", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			output, err := cli.ModelList(ctx, cfg, templateName)
			Expect(err).NotTo(HaveOccurred())
			Expect(cli.ValidateModelListOutput(output, templateName)).To(Succeed())
			fmt.Printf("[TEST] Application model list validated successfully!\n")
		})
		It("verifies application model info command", Label("spyre-independent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			output, err := cli.ModelDownload(ctx, cfg, templateName)
			Expect(err).NotTo(HaveOccurred())
			Expect(cli.ValidateModelDownloadOutput(output, templateName)).To(Succeed())
			fmt.Printf("[TEST] Application model download validated successfully!\n")
		})
	})
	Context("Bootstrap Steps", func() {
		It("runs bootstrap configure", Label("spyre-dependent"), func() {
			output, err := cli.BootstrapConfigure(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(cli.ValidateBootstrapConfigureOutput(output)).To(Succeed())
		})
		It("runs bootstrap validate", Label("spyre-dependent"), func() {
			output, err := cli.BootstrapValidate(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(cli.ValidateBootstrapValidateOutput(output)).To(Succeed())
		})
		It("runs full bootstrap", Label("spyre-dependent"), func() {
			output, err := cli.Bootstrap(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(cli.ValidateBootstrapFullOutput(output)).To(Succeed())
		})
	})
	Context("Application Image Command Tests", func() {
		It("lists images for rag template", Label("spyre-independent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			err := cli.ListImage(ctx, cfg, templateName)
			Expect(err).NotTo(HaveOccurred())
			fmt.Printf("[TEST] Images listed successfully for %s template\n", templateName)
		})
		It("pulls images for rag template", Label("spyre-independent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			err := cli.PullImage(ctx, cfg, templateName)
			Expect(err).NotTo(HaveOccurred())
			fmt.Printf("[TEST] Images pulled successfully for %s template\n", templateName)
		})
	})
	Context("Application Lifecycle", func() {
		It("creates rag application, runs health checks and validates RAG endpoints", Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
			defer cancel()

			pods := []string{"backend", "ui", "db"} // replace with actual pod names

			createOutput, err := cli.CreateAppWithHealthAndRAG(
				ctx,
				cfg,
				appName,
				templateName,
				"ui.port=3000,backend.port=5000",
				"5000", // backend port
				"3000", //ui port
				cli.CreateOptions{
					SkipImageDownload: false,
					SkipModelDownload: false,
				},
				pods,
			)
			Expect(err).NotTo(HaveOccurred())

			ragBaseURL, err = cli.GetBaseURL(createOutput, "5000")
			Expect(err).NotTo(HaveOccurred())

			judgePort := os.Getenv("LLM_JUDGE_PORT")
			if judgePort == "" {
				judgePort = "8011"
			}

			judgeBaseURL, err = cli.GetBaseURL(createOutput, judgePort)
			Expect(err).NotTo(HaveOccurred())

			fmt.Printf("[TEST] Application %s created, healthy, and RAG endpoints validated\n", appName)
		})
	})
	Context("Application Observability", func() {
		It("verifies application ps output", Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			cases := map[string][]string{
				"normal": nil,
				"wide":   {"-o", "wide"},
			}

			for name, flags := range cases {
				By(fmt.Sprintf("running application ps %s", name))

				output, err := cli.ApplicationPS(ctx, cfg, appName, flags...)
				Expect(err).NotTo(HaveOccurred())
				Expect(cli.ValidateApplicationPS(output)).To(Succeed())
			}
		})
		It("verifies application info output", Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			infoOutput, err := cli.ApplicationInfo(ctx, cfg, appName)
			Expect(err).NotTo(HaveOccurred())

			Expect(cli.ValidateApplicationInfo(infoOutput, appName, templateName)).To(Succeed())
			fmt.Printf("[TEST] Application info output validated successfully!\n")
		})
		It("Verifies pods existence, health status  and restart count", Label("spyre-dependent"), func() {
			if !podmanReady {
				Skip("Podman not available - will be installed via bootstrap configure")
			}
			err := podman.VerifyContainers(appName)
			Expect(err).NotTo(HaveOccurred(), "verify containers failed")
			fmt.Println("[TEST] Containers verified")
		})
		It("Verifies Exposed Ports of the application", Label("spyre-dependent"), func() {
			if !podmanReady {
				Skip("Podman not available - will be installed via bootstrap configure")
			}
			err := podman.VerifyExposedPorts(appName)
			Expect(err).NotTo(HaveOccurred(), "Verify exposed ports failed")
			fmt.Println("[TEST] Exposed ports verified")
		})
	})
	Context("Application Teardown", func() {
		It("stops the application", Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			suffixes, ok := mainPodsByTemplate[templateName]
			Expect(ok).To(BeTrue(), "unknown templateName")

			pods := make([]string, 0, len(suffixes))
			for _, s := range suffixes {
				pods = append(pods, fmt.Sprintf("%s--%s", appName, s))
			}

			output, err := cli.StopAppWithPods(ctx, cfg, appName, pods)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			fmt.Printf("[TEST] Application %s stopped successfully using --pod\n", appName)
		})
		It("starts application pods", Label("spyre-dependent"), func() {
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

			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())
			fmt.Printf("[TEST] Application %s started successfully\n", appName)
		})
		It("starts document ingestion pod and validates ingestion completion", Label("spyre-dependent"), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
			defer cancel()

			Expect(appName).NotTo(BeEmpty())

			Expect(ingestion.PrepareDocs(appName)).To(Succeed())

			Expect(ingestion.StartIngestion(ctx, cfg, appName)).To(Succeed())

			logs, err := ingestion.WaitForIngestionLogs(ctx, cfg, appName)
			Expect(err).ToNot(HaveOccurred())
			Expect(logs).To(ContainSubstring("Ingestion started"))
			Expect(logs).To(ContainSubstring("Processed '/var/docs/test_doc.pdf'"))

			fmt.Printf("[TEST] Ingestion completed successfully for application %s\n", appName)
		})
		Context("RAG Golden Dataset Validation", func() {
			It("validates RAG answers against golden dataset", func() {
				fmt.Println("[RAG] Starting golden dataset validation")

				cases, err := rag.LoadGoldenCSV(goldenPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(cases).NotTo(BeEmpty())

				accuracyThreshold := defaultAccuracyThreshold
				if v, err := strconv.ParseFloat(os.Getenv("RAG_ACCURACY_THRESHOLD"), 64); err == nil {
					accuracyThreshold = v
				}

				results := make([]rag.EvalResult, 0, len(cases))
				passed := 0
				total := len(cases)

				for i, tc := range cases {
					ctx, cancel := context.WithTimeout(context.Background(), 4 * time.Minute)
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
					fmt.Printf("[RAG] Evaluated question %d/%d | verdict=%s | reason=%s\n",i+1,total,verdict,reason)
				}

				accuracy := float64(passed) / float64(total)
				rag.PrintValidationSummary(results, accuracy)

				if accuracy < accuracyThreshold {
					Fail(fmt.Sprintf(
						"RAG accuracy %.2f below threshold %.2f",
						accuracy,
						accuracyThreshold,
					))
				}

				fmt.Println("[RAG] Golden dataset validation completed")
			})
		})
		It("deletes the application using --skip-cleanup", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			output, err := cli.DeleteAppSkipCleanup(ctx, cfg, appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			fmt.Printf("[TEST] Application %s deleted successfully using --skip-cleanup\n", appName)
		})
	})
})
