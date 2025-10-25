package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
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

func SearchServerTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { // Проверяем метод запроса
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("query")
	orderField := r.URL.Query().Get("order_field")
	orderByStr := r.URL.Query().Get("order_by")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Парсим order_by
	orderBy := OrderByAsIs
	if orderByStr != "" {
		var err error
		orderBy, err = strconv.Atoi(orderByStr)
		if err != nil {
			http.Error(w, "Bad order_by parameter", http.StatusBadRequest)
			return
		}
	}

	// Парсим limit
	limit := 25
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, "Bad limit parameter", http.StatusBadRequest)
			return
		}
	}

	// Парсим offset
	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			http.Error(w, "Bad offset parameter", http.StatusBadRequest)
			return
		}
	}

	// 1. Читаем и парсим XML файл
	users, err := loadUsersFromXMLTest()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 2. Фильтрация по query
	filteredUsers := filterUsers(users, query)

	// 3. Сортировка
	err = sortUsers(filteredUsers, orderField, orderBy)
	if err != nil {
		// Возвращаем ошибку в формате, который ожидает client.go
		errorResponse := map[string]string{"Error": "ErrorBadOrderField"}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	// 4. Применяем offset и limit
	result := applyPagination(filteredUsers, offset, limit)

	// 5. Отправляем ответ в JSON формате
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// loadUsersFromXML загружает пользователей из XML файла
func loadUsersFromXMLTest() ([]User, error) {
	xmlFile, err := os.Open("dataset.xml")
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %v", err)
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

	// Преобразуем XML структуры в User структуры
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

// filterUsers фильтрует пользователей по запросу
func filterUsersTest(users []User, query string) []User {
	if query == "" {
		return users
	}

	var filtered []User
	queryLower := strings.ToLower(query)

	for _, user := range users {
		if strings.Contains(strings.ToLower(user.Name), queryLower) || strings.Contains(strings.ToLower(user.About), queryLower) {
			filtered = append(filtered, user)
		}
	}
	return filtered
}

// sortUsers сортирует пользователей
func sortUsersTest(users []User, orderField string, orderBy int) error {
	if orderField == "" {
		orderField = "Name"
	}

	switch orderField {
	case "Id":
		sort.Slice(users, func(i, j int) bool {
			if orderBy == OrderByDesc { // 1
				return users[i].Id > users[j].Id
			} else if orderBy == OrderByAsc { // -1
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

// applyPagination применяет offset и limit
func applyPaginationTest(users []User, offset, limit int) []User {
	if offset >= len(users) {
		return []User{} // Если offset больше количества записей
	}

	end := offset + limit
	if end > len(users) {
		end = len(users)
	}
	return users[offset:end]
}

// ===== ТАБЛИЧНОЕ ТЕСТИРОВАНИЕ УСПЕШНЫХ КЕЙСОВ =====
func TestFindUsers_Success(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	testCases := []struct {
		name        string
		request     SearchRequest
		checkResult func(t *testing.T, resp *SearchResponse, err error)
	}{
		{
			name: "basic_all_users",
			request: SearchRequest{
				Limit:  5,
				Offset: 0,
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("213-unexpected error: %v", err)
					return
				}
				if len(resp.Users) == 0 {
					t.Error("217-expected users")
				}
			},
		},
		{
			name: "search_with_query",
			request: SearchRequest{
				Limit: 10,
				Query: "Boyd",
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("230-unexpected error: %v", err)
					return
				}
				for _, user := range resp.Users {
					hasBoyd := strings.Contains(strings.ToLower(user.Name), "boyd") || strings.Contains(strings.ToLower(user.About), "boyd")
					if !hasBoyd {
						t.Errorf("236-user %s doesn't contain 'Boyd'", user.Name)
					}
				}
			},
		},
		{
			name: "order_by_age_asc",
			request: SearchRequest{
				Limit:      5,
				OrderField: "Age",
				OrderBy:    OrderByAsc,
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("250-unexpected error: %v", err)
					return
				}
				if len(resp.Users) < 2 {
					return
				}
				for i := 0; i < len(resp.Users)-1; i++ {
					if resp.Users[i].Age > resp.Users[i+1].Age {
						t.Errorf("258-not sorted by age asc")
					}
				}
			},
		},
		{
			name: "order_by_age_desc",
			request: SearchRequest{
				Limit:      5,
				OrderField: "Age",
				OrderBy:    OrderByDesc,
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("272-unexpected error: %v", err)
					return
				}
				if len(resp.Users) < 2 {
					return
				}
				for i := 0; i < len(resp.Users)-1; i++ {
					if resp.Users[i].Age < resp.Users[i+1].Age {
						t.Errorf("280-not sorted by age desc")
					}
				}
			},
		},
		{
			name: "order_by_age_id",
			request: SearchRequest{
				Limit:      5,
				OrderField: "Id",
				OrderBy:    OrderByAsc,
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("294-unexpected error: %v", err)
				}
			},
		},
		{
			name: "order_by_name",
			request: SearchRequest{
				Limit:      5,
				OrderField: "Name",
				OrderBy:    OrderByAsc,
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("307-unexpected error: %v", err)
				}
			},
		},
		{
			name: "with_offset",
			request: SearchRequest{
				Limit:  2,
				Offset: 1,
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("331-unexpected error: %v", err)
				}
			},
		},
		{
			name: "empty_result",
			request: SearchRequest{
				Limit: 5,
				Query: "NonexistentUser12345",
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("343-unexpected error: %v", err)
					return
				}
				if len(resp.Users) != 0 {
					t.Error("347-expected empty result")
				}
			},
		},
		{
			name: "next_page_true",
			request: SearchRequest{
				Limit: 1,
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("358-unexpected error: %v", err)
					return
				}
				if !resp.NextPage {
					t.Error("362-NextPage should be true")
				}
			},
		},
		{
			name: "limit_25_capped",
			request: SearchRequest{
				Limit: 30, // будет обрезан до 25
			},
			checkResult: func(t *testing.T, resp *SearchResponse, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				// limit обрезается до 25, +1 для NextPage, итого макс 25 пользователей
				if len(resp.Users) > 25 {
					t.Errorf("expected max 25 users, got %d", len(resp.Users))
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &SearchClient{
				AccessToken: "test_token",
				URL:         testServer.URL,
			}
			resp, err := client.FindUsers(tc.request)
			tc.checkResult(t, resp, err)
		})
	}
}

// ===== ТЕСТЫ КЛИЕНТСКИХ ОШИБОК =====
func TestFindUsers_ClientErrors(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	testCases := []struct {
		name          string
		request       SearchRequest
		expectedError string
	}{
		{
			name: "negative_limit",
			request: SearchRequest{
				Limit: -1,
			},
			expectedError: "limit must be > 0",
		},
		{
			name: "negative_offset",
			request: SearchRequest{
				Limit:  5,
				Offset: -1,
			},
			expectedError: "offset must be > 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &SearchClient{
				AccessToken: "test_token",
				URL:         testServer.URL,
			}
			_, err := client.FindUsers(tc.request)
			if err == nil {
				t.Errorf("expected error: %s", tc.expectedError)
				return
			}
			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("expected error '%s', got: %v", tc.expectedError, err)
			}
		})
	}
}

// ===== ШАГ 8: СЕРВЕРНЫЕ ОШИБКИ =====

// SearchServerErrors - хендлер для эмуляции серверных ошибок

func SearchServerErrors(w http.ResponseWriter, r *http.Request) {
	errorType := r.URL.Query().Get("error_type")

	switch errorType {
	case "unauthorized":
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	case "internal_server_error":
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	case "bad_order_field":
		errorResponse := map[string]string{"Error": "ErrorBadOrderField"}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
	case "bad_json":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json {"))
	case "unknown_bad_request":
		errorResponse := map[string]string{"Error": "UnknownError"}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
	case "bad_error_json":
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid error json"))
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]User{})
	}
}

// Табличное тестирование серверных ошибок
func TestFindUsers_ServerErrors(t *testing.T) {
	errorServer := httptest.NewServer(http.HandlerFunc(SearchServerErrors))
	defer errorServer.Close()

	testCases := []struct {
		name          string
		errorType     string
		expectedError string
	}{
		{
			name:          "unauthorized",
			errorType:     "unauthorized",
			expectedError: "Bad AccessToken",
		},
		{
			name:          "internal_server_error",
			errorType:     "internal_server_error",
			expectedError: "SearchServer fatal error",
		},
		{
			name:          "bad_order_field",
			errorType:     "bad_order_field",
			expectedError: "OrderFeld",
		},
		{
			name:          "bad_json_response",
			errorType:     "bad_json",
			expectedError: "can't unpack error json",
		},
		{
			name:          "unknown_bad_request",
			errorType:     "unknown_bad_request",
			expectedError: "unknown_bad_request error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &SearchClient{
				AccessToken: "test_token",
				URL:         errorServer.URL + "?error_type=" + tc.errorType,
			}
			_, err := client.FindUsers(SearchRequest{Limit: 5})
			if err == nil {
				t.Errorf("expected error for %s", tc.name)
				return
			}
			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("expected '%s', got: %v", tc.expectedError, err)
			}
		})
	}
}

// Тест таймаута

func TestFindUsers_Timeout(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("[]"))
	})) // Почему 2 скобки и в каких случаях это нужно?

	defer slowServer.Close()

	client := &SearchClient{
		AccessToken: "test_token",
		URL:         slowServer.URL,
	}

	_, err := client.FindUsers(SearchRequest{Limit: 5})
	if err == nil {
		t.Errorf("expected timeout error")
		return
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

// Тест unknown error (невалидный URL)
func TestFindUsers_unknownError(t *testing.T) {
	client := &SearchClient{
		AccessToken: "test_token",
		URL:         "http://\x00invalidurl", // невалидный URL
	}
	_, err := client.FindUsers(SearchRequest{Limit: 5})
	if err == nil {
		t.Errorf("expected unknown error")
		return
	}
	if !strings.Contains(err.Error(), "unknown error") {
		t.Errorf("expected unknown error, got: %v", err)
	}
}

// ===== ШАГ 5: ПЕРВЫЙ ПРОСТОЙ ТЕСТ =====
// Тест 1: Пустая функция чтобы проверить что тесты запускаются
func TestDummy(t *testing.T) {
	t.Log("Тест запустился!")
}

// Тест 2: Простой запрос через SearchClient
// Создаем тестовый HTTP сервер с нашим хендлером
func TestFindUsers_Base(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServerTest))
	defer testServer.Close()

	searchClient := &SearchClient{ // Создаем SearchClient с URL тестового сервера
		AccessToken: "test_token",
		URL:         testServer.URL,
	}

	// Создаем простой запрос
	request := SearchRequest{
		Limit:  5,
		Offset: 0,
		Query:  "",
	}

	// Выполняем запрос
	response, err := searchClient.FindUsers(request)

	// Проверяем результат
	if err != nil {
		t.Errorf("Неожиданная ошибка: %v", err)
	}

	if response == nil {
		t.Errorf("Response не должен быть nil")
		return
	}

	if len(response.Users) == 0 {
		t.Error("Должны быть найдены пользователи")
		return
	}

	t.Logf("✓ Успешно найдено %d пользователей", len(response.Users))

	// Выводим первого пользователя для проверки
	if len(response.Users) > 0 {
		user := response.Users[0]
		t.Logf("✓ Первый пользователь: ID=%d, Name='%s', Age=%d",
			user.Id, user.Name, user.Age)
	}

	// Проверяем NextPage логику
	t.Logf("✓ NextPage: %t", response.NextPage)
}
