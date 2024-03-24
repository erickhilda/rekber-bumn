package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

const (
	baseURL       = "https://rekrutmenbersama2024.fhcibumn.id"
	jobURL        = baseURL + "/job"
	loadRecordURL = baseURL + "/job/loadRecord"
	getDetailURL  = baseURL + "/job/get_detail_vac"
)

func main() {
	client := &http.Client{}

	// Get CSRF token
	csrfToken := getCSRFToken(client, jobURL)
	fmt.Println("CSRF token:", csrfToken)

	// Request all jobs
	// jobs := requestAllJobs(client, csrfToken)
	// fmt.Println("Total jobs:", len(jobs.Data.Result))

	// Write jobs data to CSV
	// parseToCSV(jobs.Data.Result, "data/all_jobs.csv")

	// Get details for each job
	details := getAllDetails("data/all_jobs.csv", client, csrfToken)

	// Write details data to CSV
	var detailsData []interface{}
	for _, detail := range details {
		detailsData = append(detailsData, detail)
	}
	parseToCSV(detailsData, "data/details.csv")
}

func getCSRFToken(client *http.Client, url string) string {
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("Error getting CSRF token:", err)
		return ""
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Println("Error parsing HTML:", err)
		return ""
	}

	return doc.Find("input[name='csrf_fhci']").AttrOr("value", "")
}

type AllJobs struct {
	Data struct {
		Result []interface{} `json:"result"`
	} `json:"data"`
}

func requestAllJobs(client *http.Client, csrfToken string) AllJobs {
	formData := strings.NewReader("csrf_fhci=" + csrfToken + "&company=all")
	req, err := http.NewRequest("POST", loadRecordURL, formData)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return AllJobs{}
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "csrf_cookie_fhci="+csrfToken)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return AllJobs{}
	}
	defer resp.Body.Close()

	var result AllJobs
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println("Error decoding JSON:", err)
		return AllJobs{}
	}

	return result
}

func parseToCSV(data []interface{}, path string) {
	file, err := os.Create(path)
	if err != nil {
		fmt.Println("Error creating CSV file:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(bufio.NewWriter(file))
	defer writer.Flush()

	// Write headers
	keys := make([]string, 0)
	for k := range data[0].(map[string]interface{}) {
		keys = append(keys, k)
	}
	if err := writer.Write(keys); err != nil {
		fmt.Println("Error writing CSV headers:", err)
		return
	}

	// Write data
	for _, item := range data {
		row := make([]string, 0)
		for _, k := range keys {
			row = append(row, fmt.Sprintf("%v", item.(map[string]interface{})[k]))
		}
		if err := writer.Write(row); err != nil {
			fmt.Println("Error writing CSV row:", err)
			return
		}
	}
}

func getColumnIndex(header []string, columnName string) int {
	for i, name := range header {
		if name == columnName {
			return i
		}
	}
	return -1
}

func getAllDetails(csvPath string, client *http.Client, csrfToken string) []map[string]interface{} {
	file, err := os.Open(csvPath)
	if err != nil {
		fmt.Println("Error opening CSV file:", err)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading CSV file:", err)
		return nil
	}

	if !scanner.Scan() {
		fmt.Println("Error reading CSV header:", scanner.Err())
	}
	header := strings.Split(scanner.Text(), ",")
	fmt.Println("Header: ", header)
	columnName := "vacancy_id"
	columnIndex := getColumnIndex(header, columnName)
	if columnIndex == -1 {
		fmt.Println("Column not found:", columnName)
	}

	vacantIDs := make([]string, 0)
	for scanner.Scan() {
		record := strings.Split(scanner.Text(), ",")
		if columnIndex < len(record) {
			data := record[columnIndex]
			vacantIDs = append(vacantIDs, data)
		} else {
			fmt.Println("Column index is out of range for the current record")
		}
	}

	var wg sync.WaitGroup
	dataChan := make(chan map[string]interface{})

	for _, id := range vacantIDs[1:] {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			detail := getDetailJob(client, id, csrfToken)
			if detail != nil {
				dataChan <- detail
			}
		}(id)
	}

	go func() {
		wg.Wait()
		close(dataChan)
	}()

	data := make([]map[string]interface{}, 0)
	for detail := range dataChan {
		data = append(data, detail)
	}

	return data
}

func getDetailJob(client *http.Client, jobID string, csrfToken string) map[string]interface{} {
	formData := strings.NewReader("csrf_fhci=" + csrfToken + "&id=" + jobID)
	req, err := http.NewRequest("POST", getDetailURL, formData)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "csrf_cookie_fhci="+csrfToken)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println("Error decoding JSON:", err)
		return nil
	}

	return result
}
