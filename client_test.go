// client_test.go
package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// SearchServer для тестов
func SearchServerTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("query")
	orderField := r.URL.Query().Get("order_field")
	orderByStr := r.URL.Query().Get("order_by")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	orderBy := OrderByAsIs
	if orderByStr != "" {
		var err error
		orderBy, err = strconv.Atoi(orderByStr)
		if err != nil {
			http.Error(w, "Bad order_by parameter", http.StatusBadRequest)
			return
		}
	}

	limit := 25
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, "Bad limit parameter", http.StatusBadRequest)
			return
		}
	}

	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			http.Error(w, "Bad offset parameter", http.StatusBadRequest)
			return
		}
	}

	users, err := loadUsersFromXMLTest()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	filteredUsers := filterUsersTest(users, query)
	err = sortUsersTest(filteredUsers, orderField, orderBy)
	if err != nil {
		errorResponse := map[string]string{"Error": "ErrorBadOrderField"}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	result := applyPaginationTest(filteredUsers, offset, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func loadUsersFromXMLTest() ([]User, error) {
	xmlFile, err := os.Open("dataset.xml")
	if err != nil {
		return nil, err
	}
	defer xmlFile.Close()

	xmlData, err := io.ReadAll(xmlFile)
	if err != nil {
		return nil, err
	}

	var xmlRoot XMLRoot
	if err := xml.Unmarshal(xmlData, &xmlRoot); err != nil {
		return nil, err
	}

	var users []User
	for _, xmlUser := range xmlRoot.Users {
		user := User{
			Id:     xmlUser.ID,
			Name:   strings.TrimSpace(xmlUser.FirstName + " " + xmlUser.LastName),
			Age:    xmlUser.Age,
			About:  xmlUser.About,
			Gender: xmlUser.Gender,
		}
		users = append(users, user)
	}
	return users, nil
}

func filterUsersTest(users []User, query string) []User {
	if query == "" {
		return users
	}
	var filtered []User
	queryLower := strings.ToLower(query)
	for _, user := range users {
		if strings.Contains(strings.ToLower(user.Name), queryLower) ||
			strings.Contains(strings.ToLower(user.About), queryLower) {
			filtered = append(filtered, user)
		}
	}
	return filtered
}

func sortUsersTest(users []User, orderField string, orderBy int) error {
	if orderField == "" {
		orderField = "Name"
	}
	switch orderField {
	case "Id":
		sort.Slice(users, func(i, j int) bool {
			if orderBy == OrderByDesc {
				return users[i].Id > users[j].Id
			} else if orderBy == OrderByAsc {
				return users[i].Id < users[j].Id
			}
			return false
		})
	case "Age":
		sort.Slice(users, func(i, j int) bool {
			if orderBy == OrderByDesc {
				return users[i].Age > users[j].Age
			} else if orderBy == OrderByAsc {
				return users[i].Age < users[j].Age
			}
			return false
		})
	case "Name":
		sort.Slice(users, func(i, j int) bool {
			if orderBy == OrderByDesc {
				return users[i].Name > users[j].Name
			} else if orderBy == OrderByAsc {
				return users[i].Name < users[j].Name
			}
			return false
		})
	default:
		return errors.New("unknown order field")
	}
	return nil
}

func applyPaginationTest(users []User, offset, limit int) []User {
	if offset >= len(users) {
		return []User{}
	}
	end := offset + limit
	if end > len(users) {
		end = len(users)
	}
	return users[offset:end]
}

// Тест 1: Базовый запрос
func TestFindUsers_Basic(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         testServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 5})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(resp.Users) == 0 {
		t.Error("expected users")
	}
	t.Logf("Found %d users", len(resp.Users))
}

// Тест 2: Поиск по query
func TestFindUsers_WithQuery(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         testServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 10, Query: "Boyd"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	t.Logf("Found %d users with 'Boyd'", len(resp.Users))
}

// Тест 3: Сортировка по возрасту
func TestFindUsers_OrderByAge(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         testServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{
		Limit:      5,
		OrderField: "Age",
		OrderBy:    OrderByAsc,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	t.Logf("Sorted by age: %d users", len(resp.Users))
}

// Тест 4: Negative limit (клиентская ошибка)
func TestFindUsers_NegativeLimit(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         testServer.URL,
	}

	_, err := client.FindUsers(SearchRequest{Limit: -1})
	if err == nil {
		t.Error("expected error for negative limit")
	}
	if !strings.Contains(err.Error(), "limit must be > 0") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

// Тест 5: Negative offset (клиентская ошибка)
func TestFindUsers_NegativeOffset(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         testServer.URL,
	}

	_, err := client.FindUsers(SearchRequest{Limit: 5, Offset: -1})
	if err == nil {
		t.Error("expected error for negative offset")
	}
	if !strings.Contains(err.Error(), "offset must be > 0") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

// Тест 6: Limit больше 25 (обрезается)
func TestFindUsers_LimitCapped(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         testServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 30})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(resp.Users) > 25 {
		t.Errorf("limit should be capped at 25, got %d", len(resp.Users))
	}
	t.Logf("Limit capped correctly, got %d users", len(resp.Users))
}

// Тест 7: NextPage логика
func TestFindUsers_NextPage(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         testServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 1})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !resp.NextPage {
		t.Error("NextPage should be true when limit=1")
	}
	t.Logf("NextPage: %t", resp.NextPage)
}

// Теперь тесты ошибок через отдельный сервер
func SearchServerErrors(w http.ResponseWriter, r *http.Request) {
	errorType := r.URL.Query().Get("error_type")

	switch errorType {
	case "unauthorized":
		http.Error(w, "Unauthorized", http.StatusUnauthorized)

	case "internal":
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

	case "bad_order_field":
		errorResponse := map[string]string{"Error": "ErrorBadOrderField"}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)

	case "bad_json":
		// Возвращаем невалидный JSON со статусом 200
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json {"))

	case "bad_error_json":
		// Возвращаем невалидный JSON в ошибке
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid error json"))

	case "unknown_bad_request":
		errorResponse := map[string]string{"Error": "SomeUnknownError"}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)

	default:
		// По умолчанию возвращаем пустой валидный JSON массив
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]User{})
	}
}

// Тест 8: Unauthorized error
func TestFindUsers_Unauthorized(t *testing.T) {
	unauthorizedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer unauthorizedServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         unauthorizedServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 5})
	if err == nil {
		t.Error("expected error")
		return
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	if !strings.Contains(err.Error(), "Bad AccessToken") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

// Тест 9: Internal Server Error
func TestFindUsers_InternalError(t *testing.T) {
	internalErrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer internalErrorServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         internalErrorServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 5})
	if err == nil {
		t.Error("expected error")
		return
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	if !strings.Contains(err.Error(), "SearchServer fatal error") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

// Тест 10: Bad Order Field
func TestFindUsers_BadOrderField(t *testing.T) {
	badOrderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errorResponse := map[string]string{"Error": "ErrorBadOrderField"}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
	}))
	defer badOrderServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         badOrderServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 5, OrderField: "Invalid"})
	if err == nil {
		t.Error("expected error")
		return
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	if !strings.Contains(err.Error(), "OrderFeld") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

// Тест 11: Bad JSON response
func TestFindUsers_BadJSON(t *testing.T) {
	badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Возвращаем невалидный JSON со статусом 200
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json response"))
	}))
	defer badJSONServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         badJSONServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 5})
	if err == nil {
		t.Error("expected error")
		return
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	if !strings.Contains(err.Error(), "cant unpack result json") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

// Тест 12: Bad error JSON
func TestFindUsers_BadErrorJSON(t *testing.T) {
	badErrorJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid error json"))
	}))
	defer badErrorJSONServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         badErrorJSONServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 5})
	if err == nil {
		t.Error("expected error")
		return
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	if !strings.Contains(err.Error(), "cant unpack error json") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

// Тест 13: Unknown bad request
func TestFindUsers_UnknownBadRequest(t *testing.T) {
	unknownErrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errorResponse := map[string]string{"Error": "SomeUnknownError"}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
	}))
	defer unknownErrorServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         unknownErrorServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 5})
	if err == nil {
		t.Error("expected error")
		return
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	if !strings.Contains(err.Error(), "unknown bad request error") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

// Тест 14: Timeout
func TestFindUsers_Timeout(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("[]"))
	}))
	defer slowServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         slowServer.URL,
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 5})
	if err == nil {
		t.Error("expected timeout error")
		return
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

// Тест 15: Unknown error (bad URL)
func TestFindUsers_UnknownError(t *testing.T) {
	client := &SearchClient{
		AccessToken: "test_token",
		// URL:         "http://\x00invalid",
		// Используем несуществующий хост вместо невалидного URL
		URL: "http://127.0.0.1:0",
	}

	resp, err := client.FindUsers(SearchRequest{Limit: 5})
	if err == nil {
		t.Error("expected error")
		return
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	if !strings.Contains(err.Error(), "unknown error") {
		t.Errorf("wrong error: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}
