package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/PagerDuty/go-pagerduty/webhookv3"
	"github.com/spf13/cobra"
)

var (
	client *pagerduty.Client
	secret string
	apiKey string
)

var rootCmd = &cobra.Command{
	Use:   "NoiseNullifier",
	Short: "A bridge between PagerDuty and AlertManager",
	Long:  `This tool acts as a bridge, converting PagerDuty webhook events to AlertManager silences.`,
	Run:   execute,
}

type AlertManagerSilence struct {
	Matchers  []Matcher `json:"matchers"`
	StartsAt  string    `json:"startsAt"`
	EndsAt    string    `json:"endsAt"`
	CreatedBy string    `json:"createdBy"`
	Comment   string    `json:"comment"`
}

type AlertBody struct {
	Details   map[string]interface{} `json:"details"`
	ClientURL string                 `json:"client_url"`
}

type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
}

type WebhookV3 struct {
	Event struct {
		ID           string `json:"id"`
		EventType    string `json:"event_type"`
		ResourceType string `json:"resource_type"`
		Data         struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title"`
			// Add more fields from Data if required
		} `json:"data"`
		// Add more fields if required outside Data
	} `json:"event"`
}

func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func execute(cmd *cobra.Command, args []string) {
	// All your main function logic goes here

	// Check and load necessary environment variables
	if secret == "" || apiKey == "" {
		log.Fatalf("Required environment variables are not set")
	}

	log.Println("Starting NoiseNullifier...")

	// Setup HTTP route
	http.HandleFunc("/webhook", handleWebhook)

	// Start the HTTP server
	log.Println("Listening on :8080 for incoming webhooks")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Failed to start HTTP server:", err)
	}
}

func extractProtocolAndDomain(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host), nil
}

func sendToAlertManager(silence *AlertManagerSilence, alertManagerAddr string) error {
	log.Println("Attempting to send silence to AlertManager")
	alertManagerAPI := alertManagerAddr + "/api/v2/silences"

	data, err := json.Marshal(silence)
	if err != nil {
		return err
	}

	// Debug: Print the serialized JSON body
	log.Printf("Debug: Sending request to AlertManager with body: %s", string(data))

	req, err := http.NewRequest("POST", alertManagerAPI, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AlertManager returned non-200 status code %d: %s", resp.StatusCode, body)
	}

	log.Println("Silence successfully sent to AlertManager")
	return nil
}

func convertToAlertManagerSilence(labels map[string]string) *AlertManagerSilence {
	var matchers []Matcher

	const AcknowledgeDuration = time.Hour
	endsAt := time.Now().Add(AcknowledgeDuration)

	// Convert the labels into matchers
	for key, value := range labels {
		isRegex := strings.Contains(value, "|")
		matchers = append(matchers, Matcher{
			Name:    key,
			Value:   value,
			IsRegex: isRegex,
		})
	}

	return &AlertManagerSilence{
		Matchers:  matchers,
		StartsAt:  time.Now().Format(time.RFC3339), // Starts immediately.
		EndsAt:    endsAt.Format(time.RFC3339),
		CreatedBy: "PagerDuty-AlertManager bridge", // Fixed value; adjust if you want.
		Comment:   "Silenced by our PagerDuty-AlertManager bridge based on incident data",
	}
}

func copyRequestWithBody(r *http.Request, bodyBytes []byte) *http.Request {
	rCopy := r.Clone(r.Context())
	rCopy.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return rCopy
}

// Assuming you have the incidentID:
func getEventDetails(incidentID string) (map[string]string, string, error) {
	incidentAlerts, err := client.ListIncidentAlerts(incidentID)
	if err != nil {
		return nil, "", err
	}
	if len(incidentAlerts.Alerts) == 0 {
		return nil, "", fmt.Errorf("no alerts associated with incident %s", incidentID)
	}
	b := incidentAlerts.Alerts[0].Body
	if b == nil {
		return nil, "", fmt.Errorf("alert body is empty for incident %s", incidentID)
	}

	details, ok := b["details"].(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("failed to parse details for incident %s", incidentID)
	}

	// Debug Output for details
	fmt.Printf("DEBUG: Details for incident %s: %+v\n", incidentID, details)

	firingStr, firingOk := details["firing"].(string)
	if !firingOk || firingStr == "" {
		return nil, "", fmt.Errorf("failed to find firing details for incident %s", incidentID)
	}

	// Parse the Labels block from the firing string
	labels, err := parseLabels(firingStr)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse labels for incident %s: %s", incidentID, err)
	}

	cefDetails, cefDetailsOk := b["cef_details"].(map[string]interface{})
	if !cefDetailsOk {
		return nil, "", fmt.Errorf("failed to parse cefDetails for incident %s", incidentID)
	}

	clientURL, clientURLOk := cefDetails["client_url"].(string)
	if !clientURLOk || clientURL == "" {
		return nil, "", fmt.Errorf("failed to find client_url for incident %s", incidentID)
	}

	return labels, clientURL, nil
}

// parseLabels extracts and merges labels from the firing string.
func parseLabels(firingStr string) (map[string]string, error) {
	labelsMap := make(map[string]string)

	// Split by "Labels:" to divide data into blocks
	blocks := strings.Split(firingStr, "Labels:")

	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		// Add "Labels:" back to each block for consistent parsing
		block = "Labels:" + block

		// Extract labels from each block
		blockLabels, err := extractBlockLabels(block)
		if err != nil {
			return nil, err
		}

		// Merge block labels into the main labels map
		for key, value := range blockLabels {
			existingValue, exists := labelsMap[key]

			if exists && existingValue != value {
				labelsMap[key] = existingValue + "|" + value
			} else if !exists {
				labelsMap[key] = value
			}
		}
	}

	return labelsMap, nil
}

// extractBlockLabels extracts labels from a single block of data.
func extractBlockLabels(block string) (map[string]string, error) {
	labels := make(map[string]string)

	lines := strings.Split(block, "\n")
	isInLabelsSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Remove the prefix "- " from the line
		line = strings.TrimPrefix(line, "- ")

		// Detect start of the Labels section
		if strings.HasPrefix(line, "Labels:") {
			isInLabelsSection = true
			continue
		}

		// Detect end of the Labels section
		if strings.HasPrefix(line, "Annotations:") {
			break
		}

		// If inside the Labels section, parse the label
		if isInLabelsSection {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				labels[key] = value
			}
		}
	}

	return labels, nil
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	log.Println("Received webhook")

	// Read the entire body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading request body:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Make a copy of the request with the read body bytes
	rCopy := copyRequestWithBody(r, bodyBytes)

	// Now that we have the body content, we can safely pass it to the goroutine
	go processWebhook(rCopy)

	w.WriteHeader(http.StatusAccepted) // Immediately respond with "202 Accepted"
}

func processWebhook(r *http.Request) {
	err := webhookv3.VerifySignature(r, secret)
	if err != nil {
		log.Println("Webhook verification failed:", err)
		return
	}
	log.Println("Received signed webhook")
	// Decode the incoming webhook payload
	var pdWebhook WebhookV3
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&pdWebhook); err != nil {
		fmt.Println("Invalid request payload:", http.StatusBadRequest)
		return
	}

	// Log the entire pdWebhook content
	prettyContent, err := json.MarshalIndent(pdWebhook, "", "  ")
	if err != nil {
		log.Println("Debug: Error marshaling pdWebhook for logging:", err)
	} else {
		log.Println("Debug: Content of pdWebhook:", string(prettyContent))
	}

	// Process the webhook event
	switch pdWebhook.Event.EventType {
	case "incident.acknowledged":
		incidentID := pdWebhook.Event.Data.ID

		// Fetch the incident details using the updated method for pagerduty-go v1.7
		details, clientURL, err := getEventDetails(incidentID)
		if err != nil {
			log.Println("Error fetching event details:", err)
			return
		}

		alertManagerAddr, err := extractProtocolAndDomain(clientURL)
		if err != nil {
			log.Println("Error fetching AlertManager client URL from incident:", err)
			return
		}

		fmt.Printf("DEBUG: Details for incident %s: %+v\n", incidentID, details)
		fmt.Printf("DEBUG: AlertManager client URL from incident: %s\n", alertManagerAddr)

		silence := convertToAlertManagerSilence(details)
		if err := sendToAlertManager(silence, alertManagerAddr); err != nil {
			log.Println("Error sending to AlertManager:", err)
		}

	case "pagey.ping":
		log.Println("Received a PagerDuty ping event.")

	// ... You can add more cases as needed ...

	default:
		log.Println("Unhandled event type:", pdWebhook.Event.EventType)
	}
}

func init() {
	// This will be called before main() and is a good place to initialize
	// configuration parameters. For example, fetching environment variables.
	secret = getEnv("PD_SECRET", "")
	apiKey = getEnv("PD_APIKEY", "")
	// Initialize the PagerDuty client
	client = pagerduty.NewClient(apiKey)

	//rootCmd.PersistentFlags().StringVar(&alertManagerAddr, "alertmanager", "http://alertmanager:9093", "AlertManager API address")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
