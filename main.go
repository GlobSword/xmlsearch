package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

type XMLUser struct {
	ID        int    `xml:"id"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

type XMLRoot struct {
	Users []XMLUser `xml:"row"`
}

// SearchServer - главный хендлер поиска пользователей*
// SearchServer - своего рода внешняя система. Непосредственно занимается поиском данных в
// файле `dataset.xml`. В продакшене бы запускалась в виде отдельного веб-сервиса,
// но в вашем колде запустится как отдельный хендлер.

func SearchServer(w http.ResponseWriter, r *http.Request) {
	log.Printf("Получен запрос: %s %s", r.Method, r.URL.String()) // Логируем входящий запрос

	if r.Method != http.MethodGet { // Проверяем метод запроса
		log.Printf("Неверный метод: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем AccessToken в заголовке (как делает client.go)
	accessToken := r.Header.Get("AccessToken")
	log.Printf("AccessToken: '%s'", accessToken)

	// Получаем параметры из URL
	query := r.URL.Query().Get("query")            // что ищем
	orderField := r.URL.Query().Get("order_field") // по какому полю сортируем
	orderByStr := r.URL.Query().Get("order_by")    // используем константу из client.go (-1)
	limitStr := r.URL.Query().Get("limit")         // сколько записей вернуть
	offsetStr := r.URL.Query().Get("offset")       // сколько пропустить

	log.Printf("Параметры запроса:	")
	log.Printf("query: '%s'", query)
	log.Printf("order_field: '%s'", orderField)
	log.Printf("order_by: '%s'", orderByStr)
	log.Printf("limit: '%s'", limitStr)
	log.Printf("offset: '%s'", offsetStr)

	// Парсим order_by
	orderBy := OrderByAsIs
	if orderByStr != "" {
		var err error
		orderBy, err = strconv.Atoi(orderByStr)
		if err != nil {
			log.Printf("Ошибка парсинга order_by: %v", err)
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
			log.Printf("Ошибка парсинга limit %v", err)
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
			log.Printf("Ошибка парсинга offset: %v", err)
			http.Error(w, "Bad offset parameter", http.StatusBadRequest)
			return
		}
	}

	log.Printf("Обработанные параметры: query='%s', orderField='%s', orderBy=%d, limit=%d, offset=%d", query, orderField, orderBy, limit, offset)

	// 1. Читаем и парсим XML файл
	users, err := loadUsersFromXML()
	if err != nil {
		log.Printf("Ошибка загрузки пользователей: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("Загружено пользователей: %d", len(users))

	// 2. Фильтрация по query
	filteredUsers := filterUsers(users, query)
	log.Printf("После фильтрации: %d пользователей", len(filteredUsers))

	// 3. Сортировка
	err = sortUsers(filteredUsers, orderField, orderBy)
	if err != nil {
		log.Printf("Ошибка сортировки: %v", err)
		// Возвращаем ошибку в формате, который ожидает client.go
		errorResponse := map[string]string{"Error": "ErrorBadOrderField"}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	// 4. Применяем offset и limit
	result := applyPagination(filteredUsers, offset, limit)
	log.Printf("Итоговый результат: %d пользователей", len(result))

	// 5. Отправляем ответ в JSON формате
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("Ошибка кодирования JSON: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("Ответ успешно отправлен")
}

// loadUsersFromXML загружает пользователей из XML файла
func loadUsersFromXML() ([]User, error) {
	xmlFile, err := os.Open("dataset.xml")
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %v", err)
	}
	defer xmlFile.Close()

	xmlData, err := io.ReadAll(xmlFile)
	if err != nil {
		return nil, fmt.Errorf("не удалось распарсить XML: %v", err)
	}

	var xmlRoot XMLRoot
	if err := xml.Unmarshal(xmlData, &xmlRoot); err != nil {
		return nil, fmt.Errorf("Не удалось распарсить XML: %v", err)
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
func filterUsers(users []User, query string) []User {
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
func sortUsers(users []User, orderField string, orderBy int) error {
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
		return fmt.Errorf("неизвестное поле сортировки: %s", orderField)
	}
	return nil
}

// applyPagination применяет offset и limit
func applyPagination(users []User, offset, limit int) []User {
	if offset >= len(users) {
		return []User{} // Если offset больше количества записей
	}

	end := offset + limit
	if end > len(users) {
		end = len(users)
	}
	return users[offset:end]
}

// Дополнительный хендлер для тестирования ошибок
func ErrorServer(w http.ResponseWriter, r *http.Request) {
	errorType := r.URL.Query().Get("type")

	switch errorType {
	case "unauthorized":
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	case "internal":
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	case "bad_json":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json response"))
	case "timeout":
		w.Write([]byte("[]"))
	default:
		http.Error(w, "Unknown error type", http.StatusBadRequest)
	}

}

func main() {

	http.HandleFunc("/", SearchServer)
	http.HandleFunc("/error", ErrorServer)

	port := ":8080"

	fmt.Println("____________________")
	fmt.Println("Сервер запущен на http://localhost:8080")
	fmt.Println("____________________")
	fmt.Println("\nПримеры запросов для тестирования в браузере:")
	fmt.Println("\n 1. Все пользователи:")
	fmt.Println("	http://localhost:8080/")
	fmt.Println("\n 2. Поиск Boyd:")
	fmt.Println("	http://localhost/?query=Boyd")
	fmt.Println("\n 3. Сортировка по возрасту по убыванию:")
	fmt.Println("	http://localhost:8080/?order_field=Age&order_by=1&limit=3")
	fmt.Println("\n 4. Сортировка по имени по возрастанию:")
	fmt.Println("	http://localhost:8080/?order_field=Name&order_by=1&limit=5")
	fmt.Println("\n 5. Ошибка неверного поля сортировки:")
	fmt.Println("	http://localhost:8080/?order_field=InvalidField")
	fmt.Println("\n 6. С пагинацией:")
	fmt.Println("	http://localhost:8080/?limit=2&offset=1")
	fmt.Println("\n 7. Тест ошибки Unauthorized:")
	fmt.Println("	http://localhost:8080/error?type=unauthorized")
	fmt.Println("\n====================")
	fmt.Println("Ожидаю запросы...")
	fmt.Println("Для остановки: Ctrl+C")
	fmt.Println("======================")

	log.Fatal(http.ListenAndServe(port, nil))

}

/*



 */
