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
