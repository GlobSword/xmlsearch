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
	FirstName string `xml:"First_name"`
	LastName  string `xml:"Last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

type XMLRoot struct {
	Users []XMLUser `xml:"row"`
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	log.Printf("Получен запрос: %s %s", r.Method, r.URL.String())

	if r.Method != http.MethodGet {
		log.Printf("Неверный метод: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	accessToken := r.Header.Get("AccessToken")
	log.Printf("AccessToken: '%s'", accessToken)

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

}

func main() {

	query := "Boyd"       // что ищем
	orderField := "Name"  // по какому полю сортируем
	orderBy := OrderByAsc // используем константу из client.go (-1)
	limit := 5            // сколько записей вернуть
	offset := 0           // сколько пропустить

	fmt.Printf("Поиск с параметрами:\n")
	fmt.Printf("Query: '%s'\n", query)
	fmt.Printf("OrderField: '%s'\n", orderField)
	fmt.Printf("OrderBy: %d (OrderByAsc=%d, OrderByAsIs=%d, OrderByDesc=%d)\n",
		orderBy, OrderByAsc, OrderByAsIs, OrderByDesc)
	fmt.Printf("Limit: %d\n", limit)
	fmt.Printf("Offset: %d\n", offset)
	fmt.Println(strings.Repeat("=", 50))

	// 1. Читаем XML файл

	xmlFile, err := os.Open("dataset.xml")
	if err != nil {
		fmt.Printf("Error: open fail: %v\n", err)
		return
	}
	defer xmlFile.Close()

	// 2. Читаем содержимое файла

	xmlData, err := io.ReadAll(xmlFile)
	if err != nil {
		fmt.Printf("Error: reading fail: %v\n", err)
		return
	}

	// 3. Парсим XML

	var xmlRoot XMLRoot

	if err := xml.Unmarshal(xmlData, &xmlRoot); err != nil {
		fmt.Printf("Error: parsing fail: %v\n", err)
		return
	}

	fmt.Printf("Загружено пользователей из XML: %d\n", len(xmlRoot.Users))

	// 4. Преобразуем XML структуры в User структуры

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

	fmt.Println("Все пользователи до фильтрации: ")
	for i, user := range users {
		fmt.Printf(" %d, %s (ID: %d, Age: %d)\n", i+1, user.Name, user.Id, user.Age)
	}
	fmt.Println()

	// 5. Фильтрация по query

	var filteredUsers []User

	if query == "" {
		filteredUsers = users
		fmt.Println("Query пустой - берем всех пользователей")
	} else {
		fmt.Printf("Ищем '%s' в полях Name и About...\n", query)
		for _, user := range users {
			nameMatch := strings.Contains(strings.ToLower(user.Name), strings.ToLower(query))
			aboutMatch := strings.Contains(strings.ToLower(user.About), strings.ToLower(query))

			if nameMatch || aboutMatch {
				filteredUsers = append(filteredUsers, user)
				fmt.Printf("%s - найдено в ", user.Name)
				if nameMatch {
					fmt.Print("Name ")
				}
				if aboutMatch {
					fmt.Print("About ")
				}
				fmt.Println()
			}
		}
		fmt.Printf("После фильтрации по '%s': %d пользователей\n", query, len(filteredUsers))

	}

	// 6. Сортировка

	if orderField == "" {
		orderField = "Name"
	}

	fmt.Printf("\nСортируем по полю: '%s', направление: %d\n", orderField, orderBy)

	switch orderField {
	case "Id":
		sort.Slice(filteredUsers, func(i, j int) bool {
			if orderBy == OrderByDesc {
				return filteredUsers[i].Name > filteredUsers[j].Name
			} else if orderBy == OrderByAsc {
				return filteredUsers[i].Name < filteredUsers[j].Name
			}
			return false
		})
	case "Age":
		sort.Slice(filteredUsers, func(i, j int) bool {
			if orderBy == OrderByDesc {
				return filteredUsers[i].Age > filteredUsers[j].Age
			} else if orderBy == OrderByAsc {
				return filteredUsers[i].Age < filteredUsers[j].Age
			}
			return false
		})
	default:
		fmt.Printf("ОШИБКА: Неизвестное поле для сортировки: '%s'\n", orderField)
		fmt.Printf("Должно возвращаться: %s\n", ErrorBadOrderField)
		return
	}

	fmt.Println("После сортировки: ")
	for i, user := range filteredUsers {
		fmt.Printf(" %d. %s (ID: %d, Age: %d)\n", i+1, user.Name, user.Id, user.Age)
	}

	// 7. Применяем offset и limit

	totalFound := len(filteredUsers)
	if offset >= len(filteredUsers) {
		fmt.Printf("Offset (%d) >= общего количества (%d) - возвращаем пустой результат\n", offset, len(filteredUsers))
		filteredUsers = []User{}
	} else {
		end := offset + limit
		if end > len(filteredUsers) {
			end = len(filteredUsers)
		}
		fmt.Printf("Применяем offset=%d, limit=%d (берем записи с %d по %d)\n", offset, limit, offset, end-1)
		filteredUsers = filteredUsers[offset:end]
	}

	// 8. Выводим результат

	fmt.Printf("\nИтоговый результат (%d из %d найденных):\n", len(filteredUsers), totalFound)
	fmt.Println(strings.Repeat("=", 60))

	for i, user := range filteredUsers {
		fmt.Printf("%d. ID: %d, Name: '%s', Age: %d, Gender: %s\n",
			i+1, user.Id, user.Name, user.Age, user.Gender)
		about := user.About
		if len(about) > 100 {
			about = about[:100] + "..."
		}
		fmt.Printf(" About: %s\n", about)
		fmt.Println()
	}

	// 9. Дополнительно: выводим JSON как будет отправляться клиенту

	fmt.Println("JSON представление (как отправится клиенту):")
	fmt.Println(strings.Repeat("-", 40))
	jsonData, err := json.MarshalIndent(filteredUsers, "", " ")
	if err != nil {
		fmt.Printf("Ошибка формирования JSON: %v\n", err)
		return
	}
	fmt.Println(string(jsonData))

	// 10. Проверим логику NextPage (как в client.go)

	originalLimit := limit
	limit++

	fmt.Printf("\nЛогика NextPage:\n")
	fmt.Printf("Исходный limit: %d, увеличенный limit: %d\n", originalLimit, limit)
	fmt.Printf("Получили записей: %d\n", len(filteredUsers))

	nextPage := len(filteredUsers) == limit
	fmt.Printf("NextPage будет: %t\n", nextPage)

	if nextPage {
		fmt.Println("Значит есть еще записи - покажем кнопку 'Следующая страница'")
	} else {
		fmt.Println("Это последняя страница")
	}
}

/*

* SearchServer - своего рода внешняя система. Непосредственно занимается поиском данных в
файле `dataset.xml`. В продакшене бы запускалась в виде отдельного веб-сервиса,
но в вашем колде запустится как отдельный хендлер.

*/
