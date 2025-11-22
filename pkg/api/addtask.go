package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/vita269/final-boss/pkg/dates"
	"github.com/vita269/final-boss/pkg/db"
)

const dateFormat = "20060102"

type TasksResp struct {
	Tasks []*db.Task `json:"tasks"`
}

func writeJson(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)

	if err := encoder.Encode(data); err != nil {
		log.Printf("Ошибка при кодировании JSON: %v", err)
	}
}
func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJson(w, map[string]string{"error": "Метод " + r.Method + " не поддерживается. Используйте POST"}, http.StatusMethodNotAllowed)
		return
	}

	var task db.Task

	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		writeJson(w, map[string]string{"error": "некорректный JSON: " + err.Error()}, http.StatusBadRequest)
		return
	}
	log.Printf("Received task: date=%q, title=%q, repeat=%q", task.Date, task.Title, task.Repeat)

	if task.Title == "" {
		writeJson(w, map[string]string{"error": "поле title обязательно"}, http.StatusBadRequest)
		return
	}

	if err := db.CheckDate(&task); err != nil {
		writeJson(w, map[string]string{"error": err.Error()}, http.StatusBadRequest)
		return
	}
	id, err := db.AddTask(&task)
	if err != nil {
		writeJson(w, map[string]string{"error": "ошибка при добавлении задачи: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	writeJson(w, map[string]int64{"id": id}, http.StatusCreated)
}
func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJson(w, map[string]string{"error": "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	search := r.URL.Query().Get("search")

	// Логируем поисковый запрос для отладки
	if search != "" {
		log.Printf("Search request: %q", search)
	}

	tasks, err := db.Tasks(50, search)
	if err != nil {
		// Используем вашу функцию для возврата ошибки в JSON
		writeJson(w, map[string]string{"error": "ошибка при получении задач: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	resp := TasksResp{
		Tasks: tasks,
	}

	// Логируем для отладки
	log.Printf("Returning %d tasks", len(tasks))

	writeJson(w, resp, http.StatusOK)
}

func getTaskHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJson(w, map[string]string{"error": "Не указан идентификатор"}, http.StatusBadRequest)
		return
	}

	task, err := db.GetTask(id)
	if err != nil {
		writeJson(w, map[string]string{"error": err.Error()}, http.StatusNotFound)
		return
	}

	writeJson(w, task, http.StatusOK)
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJson(w, map[string]string{"error": "Метод " + r.Method + " не поддерживается. Используйте PUT"}, http.StatusMethodNotAllowed)
		return
	}

	var task db.Task

	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		writeJson(w, map[string]string{"error": "некорректный JSON: " + err.Error()}, http.StatusBadRequest)
		return
	}
	log.Printf("PUT /api/task received: id=%s, title=%s, date=%s, comment=%s, repeat=%s",
		task.ID, task.Title, task.Date, task.Comment, task.Repeat)

	if task.ID == "" {
		writeJson(w, map[string]string{"error": "поле id обязательно"}, http.StatusBadRequest)
		return
	}

	// Проверяем обязательные поля
	if task.Title == "" {
		writeJson(w, map[string]string{"error": "поле title обязательно"}, http.StatusBadRequest)
		return
	}

	// Обработка "today"
	if task.Date == "today" {
		task.Date = time.Now().Format(dateFormat)
	}

	// Проверяем дату
	if err := db.CheckDate(&task); err != nil {
		writeJson(w, map[string]string{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	// Обновляем задачу в базе
	err := db.UpdateTask(&task)
	if err != nil {
		writeJson(w, map[string]string{"error": "ошибка при обновлении задачи: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	// Возвращаем пустой JSON при успехе
	writeJson(w, map[string]interface{}{}, http.StatusOK)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJson(w, map[string]string{"error": "Метод " + r.Method + " не поддерживается. Используйте DELETE"}, http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJson(w, map[string]string{"error": "Не указан идентификатор"}, http.StatusBadRequest)
		return
	}

	err := db.DeleteTask(id)
	if err != nil {
		writeJson(w, map[string]string{"error": err.Error()}, http.StatusInternalServerError)
		return
	}

	writeJson(w, map[string]interface{}{}, http.StatusOK)
}

func doneTaskHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJson(w, map[string]string{"error": "Не указан идентификатор"}, http.StatusBadRequest)
		return
	}
	log.Printf("Выполнение задачи ID=%s", id)
	// Получаем задачу из базы
	task, err := db.GetTask(id)
	if err != nil {
		writeJson(w, map[string]string{"error": err.Error()}, http.StatusNotFound)
		return
	}
	log.Printf("Задача: date=%s, repeat=%s", task.Date, task.Repeat)
	// Если задача не повторяется - удаляем её
	if task.Repeat == "" {

		err = db.DeleteTask(id)
		if err != nil {
			writeJson(w, map[string]string{"error": "Ошибка при удалении задачи: " + err.Error()}, http.StatusInternalServerError)
			return
		}
		log.Printf("Одноразовая задача %s выполнена и удалена", id)
	} else {

		// Если задача повторяется - вычисляем следующую дату
		nextDate, err := dates.CalculateNextDateForTask(time.Now(), task.Date, task.Repeat)
		if err != nil {
			writeJson(w, map[string]string{"error": "Ошибка при расчете следующей даты: " + err.Error()}, http.StatusInternalServerError)
			return
		}

		nextDateStr := nextDate.Format(dateFormat)
		log.Printf("Следующая дата: %s -> %s", task.Date, nextDateStr)
		// Обновляем дату задачи
		err = db.UpdateDate(id, nextDateStr)
		if err != nil {
			writeJson(w, map[string]string{"error": "Ошибка при обновлении даты: " + err.Error()}, http.StatusInternalServerError)
			return
		}
		log.Printf("Повторяющаяся задача %s выполнена, следующая дата: %s", id, nextDateStr)
	}

	writeJson(w, map[string]interface{}{}, http.StatusOK)
}

func signinHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJson(w, map[string]string{"error": "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJson(w, map[string]string{"error": "invalid JSON"}, http.StatusBadRequest)
		return
	}

	var todoPassword string
	if todoPassword == "" {
		writeJson(w, map[string]string{"error": "authentication not configured"}, http.StatusInternalServerError)
		return
	}

	if req.Password != todoPassword {
		writeJson(w, map[string]string{"error": "invalid password"}, http.StatusUnauthorized)
		return
	}

	token, err := GenerateToken(req.Password)
	if err != nil {
		writeJson(w, map[string]string{"error": "failed to generate token"}, http.StatusInternalServerError)
		return
	}

	writeJson(w, map[string]string{"token": token}, http.StatusOK)
}

func nextDateHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	nowStr := r.FormValue("now")
	dateStr := r.FormValue("date")
	repeat := r.FormValue("repeat")

	var now time.Time
	if nowStr == "" {
		now = time.Now()
	} else {
		var err error
		now, err = time.Parse(dateFormat, nowStr)
		if err != nil {
			http.Error(w, "некорректный формат параметра now", http.StatusBadRequest)
			return
		}
	}

	if dateStr == "" || repeat == "" {
		http.Error(w, "обязательные параметры date и repeat не указаны", http.StatusBadRequest)
		return
	}

	result, err := dates.NextDate(now, dateStr, repeat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write([]byte(result))
}
