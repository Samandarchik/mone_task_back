package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfnt/resize"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"

	_ "taskmanager/docs"
)

// @title Task Management API
// @version 1.0
// @description Task Management API with Categories, Tasks and Task Items (JSON Storage)
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:1212
// @BasePath /
// @schemes http

// Models
type Category struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type Task struct {
	ID         string     `json:"id"`
	CategoryID string     `json:"category_id"`
	Name       string     `json:"name"`
	IsSuccess  bool       `json:"is_success"`
	Price      *float32   `json:"price"`
	Position   int        `json:"position"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
	Category   *Category  `json:"category,omitempty"`
	Items      []TaskItem `json:"items,omitempty"`
}

type TaskItem struct {
	ID       string    `json:"id"`
	TaskID   string    `json:"task_id"`
	Type     string    `json:"type"`
	Data     string    `json:"data"`
	Time     time.Time `json:"time"`
	Position int       `json:"position"`
}

// Database structure
type Database struct {
	Categories []Category `json:"categories"`
	Tasks      []Task     `json:"tasks"`
	TaskItems  []TaskItem `json:"task_items"`
}

// Response structures
type TaskItemResponse struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Position int    `json:"position"`
	Data     struct {
		ID   string    `json:"id"`
		Data string    `json:"data"`
		Time time.Time `json:"time"`
	} `json:"data"`
}

type TaskResponse struct {
	ID         string             `json:"id"`
	CategoryID string             `json:"category_id"`
	Name       string             `json:"name"`
	IsSuccess  bool               `json:"is_success"`
	Price      *float32           `json:"price"`
	Position   int                `json:"position"`
	DeletedAt  *time.Time         `json:"deleted_at,omitempty"`
	Category   []Category         `json:"category"`
	TaskName   []TaskItemResponse `json:"task_name"`
}

type UploadData struct {
	ID          string `json:"id"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	DurationMs  *int   `json:"duration_ms"`
}

type UploadResponse struct {
	Success    bool       `json:"success"`
	StatusCode int        `json:"statusCode"`
	Message    string     `json:"message"`
	Data       UploadData `json:"data"`
}

// Global variables
var (
	db       Database
	dbMutex  sync.RWMutex
	dataFile = "data/database.json"
)

// Database operations
func loadDatabase() error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	data, err := ioutil.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			db = Database{
				Categories: []Category{},
				Tasks:      []Task{},
				TaskItems:  []TaskItem{},
			}
			return saveDatabase()
		}
		return err
	}

	return json.Unmarshal(data, &db)
}

func saveDatabase() error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dataFile, data, 0644)
}

func saveDatabaseAsync() {
	go func() {
		dbMutex.Lock()
		defer dbMutex.Unlock()
		saveDatabase()
	}()
}

// Helper functions
func findCategoryByID(id string) *Category {
	for i := range db.Categories {
		if db.Categories[i].ID == id {
			return &db.Categories[i]
		}
	}
	return nil
}

func findTaskByID(id string, includeDeleted bool) *Task {
	for i := range db.Tasks {
		if db.Tasks[i].ID == id {
			if !includeDeleted && db.Tasks[i].DeletedAt != nil {
				return nil
			}
			return &db.Tasks[i]
		}
	}
	return nil
}

func findTaskItemByID(id string) *TaskItem {
	for i := range db.TaskItems {
		if db.TaskItems[i].ID == id {
			return &db.TaskItems[i]
		}
	}
	return nil
}

func getTaskItemsByID(taskID string) []TaskItem {
	var items []TaskItem
	for _, item := range db.TaskItems {
		if item.TaskID == taskID {
			items = append(items, item)
		}
	}
	return items
}

func deleteCategoryByID(id string) bool {
	for i := range db.Categories {
		if db.Categories[i].ID == id {
			db.Categories = append(db.Categories[:i], db.Categories[i+1:]...)
			return true
		}
	}
	return false
}

func deleteTaskByID(id string) bool {
	for i := range db.Tasks {
		if db.Tasks[i].ID == id {
			db.Tasks = append(db.Tasks[:i], db.Tasks[i+1:]...)
			return true
		}
	}
	return false
}

func deleteTaskItemByID(id string) bool {
	for i := range db.TaskItems {
		if db.TaskItems[i].ID == id {
			db.TaskItems = append(db.TaskItems[:i], db.TaskItems[i+1:]...)
			return true
		}
	}
	return false
}

func convertToTaskResponse(task Task) TaskResponse {
	response := TaskResponse{
		ID:         task.ID,
		CategoryID: task.CategoryID,
		Name:       task.Name,
		IsSuccess:  task.IsSuccess,
		Price:      task.Price,
		Position:   task.Position,
		DeletedAt:  task.DeletedAt,
		Category:   []Category{},
		TaskName:   []TaskItemResponse{},
	}

	// Add category
	cat := findCategoryByID(task.CategoryID)
	if cat != nil {
		response.Category = append(response.Category, *cat)
	}

	// Add items with position
	items := getTaskItemsByID(task.ID)
	for i, item := range items {
		itemResp := TaskItemResponse{
			ID:       item.ID,
			Type:     item.Type,
			Position: i + 1, // Auto position based on order
		}
		itemResp.Data.ID = item.ID
		itemResp.Data.Data = item.Data
		itemResp.Data.Time = item.Time
		response.TaskName = append(response.TaskName, itemResp)
	}

	return response
}

func decodeImage(file multipart.File, ext string) (image.Image, string, error) {
	file.Seek(0, 0)

	// HEIC/HEIF not supported in this version
	if ext == ".heic" || ext == ".heif" {
		return nil, "", fmt.Errorf("HEIC/HEIF format is not supported. Please convert to JPG/PNG")
	}

	// Try WebP
	if ext == ".webp" {
		img, err := webp.Decode(file)
		if err == nil {
			return img, "webp", nil
		}
	}

	// Try BMP
	if ext == ".bmp" {
		img, err := bmp.Decode(file)
		if err == nil {
			return img, "bmp", nil
		}
	}

	// Try TIFF
	if ext == ".tiff" || ext == ".tif" {
		img, err := tiff.Decode(file)
		if err == nil {
			return img, "tiff", nil
		}
	}

	// Try standard image formats (JPEG, PNG, GIF)
	file.Seek(0, 0)
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, "", fmt.Errorf("image decode error: %v", err)
	}

	return img, format, nil
}

func saveImage(img image.Image, savePath string, originalExt string) error {
	out, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("file creation error: %v", err)
	}
	defer out.Close()

	switch strings.ToLower(originalExt) {
	case ".png":
		return png.Encode(out, img)
	case ".gif":
		return gif.Encode(out, img, nil)
	case ".bmp":
		return bmp.Encode(out, img)
	case ".tiff", ".tif":
		return tiff.Encode(out, img, nil)
	default:
		opts := &jpeg.Options{Quality: 90}
		return jpeg.Encode(out, img, opts)
	}
}

func main() {
	// Load database
	if err := loadDatabase(); err != nil {
		log.Fatal("Database yuklashda xatolik:", err)
	}
	log.Println("✓ Database muvaffaqiyatli yuklandi")

	// Create uploads directory
	os.MkdirAll("uploads", os.ModePerm)

	r := gin.Default()
	r.Static("/static", "./uploads")

	// Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Category routes
	r.POST("/categories", createCategory)
	r.GET("/categories", getCategories)
	r.GET("/categories/:id", getCategory)
	r.PUT("/categories/:id", updateCategory)
	r.DELETE("/categories/:id", deleteCategory)

	// Task routes
	r.POST("/tasks", createTask)
	r.GET("/tasks", getTasks)
	r.GET("/tasks/deleted", getDeletedTasks)
	r.GET("/tasks/:id", getTask)
	r.PUT("/tasks/:id", updateTask)
	r.PUT("/tasks/:id/position", updateTaskPosition)
	r.DELETE("/tasks/:id", deleteTask)
	r.PUT("/tasks/:id/restore", restoreTask)
	r.DELETE("/tasks/:id/permanent", permanentDeleteTask)
	r.PUT("/tasks/:id/success", markTaskSuccess)

	// Task Item routes
	r.POST("/task-items", createTaskItem)
	r.GET("/task-items", getTaskItems)
	r.GET("/task-items/:id", getTaskItem)
	r.GET("/tasks/:id/items", getTaskItemsByTaskID)
	r.PUT("/task-items/:id", updateTaskItem)
	r.DELETE("/task-items/:id", deleteTaskItem)

	// File upload routes
	r.POST("/upload/image", uploadImage)
	r.POST("/upload/audio", uploadAudio)
	r.POST("/upload/video", uploadVideo)

	log.Println("🚀 Server running on :1212")
	log.Println("📚 Swagger: http://localhost:1212/swagger/index.html")
	r.Run(":1212")
}

// Category handlers

// @Summary Create a new category
// @Description Create a new category with the provided data
// @Tags categories
// @Accept json
// @Produce json
// @Param category body Category true "Category data"
// @Success 201 {object} Category
// @Failure 400 {object} map[string]string
// @Router /categories [post]
func createCategory(c *gin.Context) {
	var category Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dbMutex.Lock()
	category.ID = uuid.New().String()
	db.Categories = append(db.Categories, category)
	saveDatabase()
	dbMutex.Unlock()

	c.JSON(201, category)
}

// @Summary Get all categories
// @Description Get a list of all categories
// @Tags categories
// @Produce json
// @Success 200 {array} Category
// @Router /categories [get]
func getCategories(c *gin.Context) {
	dbMutex.RLock()
	defer dbMutex.RUnlock()
	c.JSON(200, db.Categories)
}

// @Summary Get category by ID
// @Description Get a single category by its ID
// @Tags categories
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} Category
// @Failure 404 {object} map[string]string
// @Router /categories/{id} [get]
func getCategory(c *gin.Context) {
	id := c.Param("id")
	dbMutex.RLock()
	defer dbMutex.RUnlock()

	cat := findCategoryByID(id)
	if cat == nil {
		c.JSON(404, gin.H{"error": "Category not found"})
		return
	}
	c.JSON(200, cat)
}

// @Summary Update category
// @Description Update an existing category by ID
// @Tags categories
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Param category body Category true "Updated category data"
// @Success 200 {object} Category
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /categories/{id} [put]
func updateCategory(c *gin.Context) {
	id := c.Param("id")

	var input Category
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	cat := findCategoryByID(id)
	if cat == nil {
		c.JSON(404, gin.H{"error": "Category not found"})
		return
	}

	cat.Data = input.Data
	saveDatabase()

	c.JSON(200, cat)
}

// @Summary Delete category
// @Description Delete a category by ID
// @Tags categories
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /categories/{id} [delete]
func deleteCategory(c *gin.Context) {
	id := c.Param("id")

	dbMutex.Lock()
	defer dbMutex.Unlock()

	if !deleteCategoryByID(id) {
		c.JSON(404, gin.H{"error": "Category not found"})
		return
	}

	saveDatabase()
	c.JSON(200, gin.H{"message": "Category deleted"})
}

// Task handlers

// @Summary Create a new task
// @Description Create a new task with category, name, and optional position
// @Tags tasks
// @Accept json
// @Produce json
// @Param task body object{category_id=string,name=string,is_success=bool,price=number,position=int} true "Task data"
// @Success 201 {object} TaskResponse
// @Failure 400 {object} map[string]string
// @Router /tasks [post]
func createTask(c *gin.Context) {
	var input struct {
		CategoryID string   `json:"category_id"`
		Name       string   `json:"name"`
		IsSuccess  bool     `json:"is_success"`
		Price      *float32 `json:"price"`
		Position   *int     `json:"position"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	task := Task{
		ID:         uuid.New().String(),
		CategoryID: input.CategoryID,
		Name:       input.Name,
		IsSuccess:  input.IsSuccess,
		Price:      input.Price,
	}

	if input.Position == nil {
		maxPos := -1
		for _, t := range db.Tasks {
			if t.DeletedAt == nil && t.Position > maxPos {
				maxPos = t.Position
			}
		}
		task.Position = maxPos + 1
	} else {
		task.Position = *input.Position
		for i := range db.Tasks {
			if db.Tasks[i].DeletedAt == nil && db.Tasks[i].Position >= *input.Position {
				db.Tasks[i].Position++
			}
		}
	}

	db.Tasks = append(db.Tasks, task)
	saveDatabase()

	response := convertToTaskResponse(task)
	c.JSON(201, response)
}

// @Summary Get all tasks
// @Description Get a list of all active tasks ordered by position
// @Tags tasks
// @Produce json
// @Success 200 {array} TaskResponse
// @Router /tasks [get]
func getTasks(c *gin.Context) {
	dbMutex.RLock()
	defer dbMutex.RUnlock()

	var responses []TaskResponse
	for _, task := range db.Tasks {
		if task.DeletedAt == nil {
			responses = append(responses, convertToTaskResponse(task))
		}
	}

	c.JSON(200, responses)
}

// @Summary Get deleted tasks
// @Description Get a list of all soft-deleted tasks
// @Tags tasks
// @Produce json
// @Success 200 {array} TaskResponse
// @Router /tasks/deleted [get]
func getDeletedTasks(c *gin.Context) {
	dbMutex.RLock()
	defer dbMutex.RUnlock()

	var responses []TaskResponse
	for _, task := range db.Tasks {
		if task.DeletedAt != nil {
			responses = append(responses, convertToTaskResponse(task))
		}
	}

	c.JSON(200, responses)
}

// @Summary Get task by ID
// @Description Get a single task by its ID
// @Tags tasks
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} TaskResponse
// @Failure 404 {object} map[string]string
// @Router /tasks/{id} [get]
func getTask(c *gin.Context) {
	id := c.Param("id")

	dbMutex.RLock()
	defer dbMutex.RUnlock()

	task := findTaskByID(id, false)
	if task == nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	response := convertToTaskResponse(*task)
	c.JSON(200, response)
}

// @Summary Update task
// @Description Update an existing task
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param task body object{category_id=string,name=string,is_success=bool,price=number} true "Updated task data"
// @Success 200 {object} TaskResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /tasks/{id} [put]
func updateTask(c *gin.Context) {
	id := c.Param("id")

	var input struct {
		CategoryID string   `json:"category_id"`
		Name       string   `json:"name"`
		IsSuccess  bool     `json:"is_success"`
		Price      *float32 `json:"price"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	task := findTaskByID(id, false)
	if task == nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	task.CategoryID = input.CategoryID
	task.Name = input.Name
	task.IsSuccess = input.IsSuccess
	task.Price = input.Price

	saveDatabase()

	response := convertToTaskResponse(*task)
	c.JSON(200, response)
}

// @Summary Update task position
// @Description Change the position of a task in the list
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param position body object{position=int} true "New position"
// @Success 200 {object} TaskResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /tasks/{id}/position [put]
func updateTaskPosition(c *gin.Context) {
	id := c.Param("id")

	var input struct {
		Position int `json:"position"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	task := findTaskByID(id, false)
	if task == nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	oldPos := task.Position
	newPos := input.Position

	if oldPos != newPos {
		if oldPos < newPos {
			for i := range db.Tasks {
				if db.Tasks[i].DeletedAt == nil && db.Tasks[i].Position > oldPos && db.Tasks[i].Position <= newPos {
					db.Tasks[i].Position--
				}
			}
		} else {
			for i := range db.Tasks {
				if db.Tasks[i].DeletedAt == nil && db.Tasks[i].Position >= newPos && db.Tasks[i].Position < oldPos {
					db.Tasks[i].Position++
				}
			}
		}
		task.Position = newPos
	}

	saveDatabase()

	response := convertToTaskResponse(*task)
	c.JSON(200, response)
}

// @Summary Soft delete task
// @Description Soft delete a task (can be restored later)
// @Tags tasks
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /tasks/{id} [delete]
func deleteTask(c *gin.Context) {
	id := c.Param("id")

	dbMutex.Lock()
	defer dbMutex.Unlock()

	task := findTaskByID(id, false)
	if task == nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	now := time.Now()
	task.DeletedAt = &now

	for i := range db.Tasks {
		if db.Tasks[i].DeletedAt == nil && db.Tasks[i].Position > task.Position {
			db.Tasks[i].Position--
		}
	}

	saveDatabase()
	c.JSON(200, gin.H{"message": "Task deleted (soft delete)"})
}

// @Summary Restore deleted task
// @Description Restore a soft-deleted task
// @Tags tasks
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /tasks/{id}/restore [put]
func restoreTask(c *gin.Context) {
	id := c.Param("id")

	dbMutex.Lock()
	defer dbMutex.Unlock()

	task := findTaskByID(id, true)
	if task == nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	if task.DeletedAt == nil {
		c.JSON(400, gin.H{"error": "Task is not deleted"})
		return
	}

	maxPos := -1
	for _, t := range db.Tasks {
		if t.DeletedAt == nil && t.Position > maxPos {
			maxPos = t.Position
		}
	}
	task.Position = maxPos + 1
	task.DeletedAt = nil

	saveDatabase()

	response := convertToTaskResponse(*task)
	c.JSON(200, gin.H{"message": "Task restored", "task": response})
}

// @Summary Permanently delete task
// @Description Permanently delete a task and all its items
// @Tags tasks
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /tasks/{id}/permanent [delete]
func permanentDeleteTask(c *gin.Context) {
	id := c.Param("id")

	dbMutex.Lock()
	defer dbMutex.Unlock()

	task := findTaskByID(id, true)
	if task == nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	// Delete files
	items := getTaskItemsByID(id)
	for _, item := range items {
		if item.Data != "" {
			oldPath := strings.TrimPrefix(item.Data, "/static/")
			os.Remove(filepath.Join("uploads", oldPath))
		}
	}

	// Delete task items
	newItems := []TaskItem{}
	for _, item := range db.TaskItems {
		if item.TaskID != id {
			newItems = append(newItems, item)
		}
	}
	db.TaskItems = newItems

	// Delete task
	deleteTaskByID(id)
	saveDatabase()

	c.JSON(200, gin.H{"message": "Task permanently deleted"})
}

// @Summary Mark task as success
// @Description Update task success status and price
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param success body object{is_success=bool,price=number} true "Success status and price"
// @Success 200 {object} TaskResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /tasks/{id}/success [put]
func markTaskSuccess(c *gin.Context) {
	id := c.Param("id")

	var input struct {
		IsSuccess bool     `json:"is_success"`
		Price     *float32 `json:"price"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	task := findTaskByID(id, false)
	if task == nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	task.IsSuccess = input.IsSuccess
	task.Price = input.Price

	saveDatabase()

	response := convertToTaskResponse(*task)
	c.JSON(200, response)
}

// TaskItem handlers

// @Summary Create task item
// @Description Create a new task item
// @Tags task-items
// @Accept json
// @Produce json
// @Param item body TaskItem true "Task item data"
// @Success 201 {object} TaskItem
// @Failure 400 {object} map[string]string
// @Router /task-items [post]
func createTaskItem(c *gin.Context) {
	var item TaskItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dbMutex.Lock()
	item.ID = uuid.New().String()
	item.Time = time.Now()

	// Auto-assign position based on existing items for this task
	maxPos := 0
	for _, existingItem := range db.TaskItems {
		if existingItem.TaskID == item.TaskID && existingItem.Position > maxPos {
			maxPos = existingItem.Position
		}
	}
	item.Position = maxPos + 1

	db.TaskItems = append(db.TaskItems, item)
	saveDatabase()
	dbMutex.Unlock()

	c.JSON(201, item)
}

// @Summary Get all task items
// @Description Get a list of all task items
// @Tags task-items
// @Produce json
// @Success 200 {array} TaskItem
// @Router /task-items [get]
func getTaskItems(c *gin.Context) {
	dbMutex.RLock()
	defer dbMutex.RUnlock()
	c.JSON(200, db.TaskItems)
}

// @Summary Get task item by ID
// @Description Get a single task item by ID
// @Tags task-items
// @Produce json
// @Param id path string true "Task item ID"
// @Success 200 {object} TaskItem
// @Failure 404 {object} map[string]string
// @Router /task-items/{id} [get]
func getTaskItem(c *gin.Context) {
	id := c.Param("id")

	dbMutex.RLock()
	defer dbMutex.RUnlock()

	item := findTaskItemByID(id)
	if item == nil {
		c.JSON(404, gin.H{"error": "Task item not found"})
		return
	}

	c.JSON(200, item)
}

// @Summary Get task items by task ID
// @Description Get all items for a specific task
// @Tags task-items
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {array} TaskItem
// @Router /tasks/{id}/items [get]
func getTaskItemsByTaskID(c *gin.Context) {
	taskID := c.Param("id")

	dbMutex.RLock()
	defer dbMutex.RUnlock()

	items := getTaskItemsByID(taskID)
	c.JSON(200, items)
}

// @Summary Update task item
// @Description Update an existing task item
// @Tags task-items
// @Accept json
// @Produce json
// @Param id path string true "Task item ID"
// @Param item body TaskItem true "Updated task item data"
// @Success 200 {object} TaskItem
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /task-items/{id} [put]
func updateTaskItem(c *gin.Context) {
	id := c.Param("id")

	var input TaskItem
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	item := findTaskItemByID(id)
	if item == nil {
		c.JSON(404, gin.H{"error": "Task item not found"})
		return
	}

	item.Type = input.Type
	item.Data = input.Data
	item.TaskID = input.TaskID

	saveDatabase()
	c.JSON(200, item)
}

// @Summary Delete task item
// @Description Delete a task item and its associated file
// @Tags task-items
// @Produce json
// @Param id path string true "Task item ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /task-items/{id} [delete]
func deleteTaskItem(c *gin.Context) {
	id := c.Param("id")

	dbMutex.Lock()
	defer dbMutex.Unlock()

	item := findTaskItemByID(id)
	if item == nil {
		c.JSON(404, gin.H{"error": "Task item not found"})
		return
	}

	if item.Data != "" {
		oldPath := strings.TrimPrefix(item.Data, "/static/")
		os.Remove(filepath.Join("uploads", oldPath))
	}

	deleteTaskItemByID(id)
	saveDatabase()

	c.JSON(200, gin.H{"message": "Task item deleted"})
}

// File upload handlers

// @Summary Upload image
// @Description Upload an image file (JPEG, PNG, GIF, BMP, TIFF, WebP). HEIC/HEIF not supported.
// @Tags uploads
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "Image file"
// @Success 200 {object} UploadResponse
// @Failure 400 {object} UploadResponse
// @Failure 500 {object} UploadResponse
// @Router /upload/image [post]
func uploadImage(c *gin.Context) {
	file, handler, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(400, UploadResponse{
			Success:    false,
			StatusCode: 400,
			Message:    "Rasmni olishda xatolik: " + err.Error(),
		})
		return
	}
	defer file.Close()

	originalExt := strings.ToLower(filepath.Ext(handler.Filename))

	img, format, err := decodeImage(file, originalExt)
	if err != nil {
		c.JSON(400, UploadResponse{
			Success:    false,
			StatusCode: 400,
			Message:    "Rasmni o'qishda xatolik: " + err.Error(),
		})
		return
	}

	log.Printf("Image decoded successfully. Format: %s, Original ext: %s", format, originalExt)

	fileID := uuid.New().String()

	saveExt := originalExt
	contentType := ""

	switch originalExt {
	case ".jpg", ".jpeg":
		saveExt = ".jpg"
		contentType = "image/jpeg"
	case ".png":
		saveExt = ".png"
		contentType = "image/png"
	case ".gif":
		saveExt = ".gif"
		contentType = "image/gif"
	case ".bmp":
		saveExt = ".bmp"
		contentType = "image/bmp"
	case ".tiff", ".tif":
		saveExt = ".tiff"
		contentType = "image/tiff"
	case ".webp":
		saveExt = ".jpg"
		contentType = "image/jpeg"
	case ".heic", ".heif":
		saveExt = ".jpg"
		contentType = "image/jpeg"
	default:
		saveExt = ".jpg"
		contentType = "image/jpeg"
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	if width > 2048 {
		img = resize.Resize(2048, 0, img, resize.Lanczos3)
		log.Println("Image resized to 2048px width")
	}

	savePath := fmt.Sprintf("uploads/%s%s", fileID, saveExt)

	err = saveImage(img, savePath, saveExt)
	if err != nil {
		c.JSON(500, UploadResponse{
			Success:    false,
			StatusCode: 500,
			Message:    "Rasmni saqlashda xatolik: " + err.Error(),
		})
		return
	}

	fileInfo, err := os.Stat(savePath)
	if err != nil {
		c.JSON(500, UploadResponse{
			Success:    false,
			StatusCode: 500,
			Message:    "Fayl ma'lumotlarini olishda xatolik: " + err.Error(),
		})
		return
	}
	fileSize := fileInfo.Size()

	imageURL := fmt.Sprintf("/static/%s%s", fileID, saveExt)

	c.JSON(200, UploadResponse{
		Success:    true,
		StatusCode: 200,
		Message:    "Rasm muvaffaqiyatli yuklandi",
		Data: UploadData{
			ID:          fileID,
			Size:        fileSize,
			URL:         imageURL,
			FileName:    handler.Filename,
			ContentType: contentType,
			DurationMs:  nil,
		},
	})
}

// @Summary Upload audio
// @Description Upload an audio file in any format
// @Tags uploads
// @Accept multipart/form-data
// @Produce json
// @Param audio formData file true "Audio file"
// @Success 200 {object} UploadResponse
// @Failure 400 {object} UploadResponse
// @Failure 500 {object} UploadResponse
// @Router /upload/audio [post]
func uploadAudio(c *gin.Context) {
	file, handler, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(400, UploadResponse{
			Success:    false,
			StatusCode: 400,
			Message:    "Audioni olishda xatolik: " + err.Error(),
		})
		return
	}
	defer file.Close()

	fileID := uuid.New().String()
	ext := strings.ToLower(filepath.Ext(handler.Filename))

	if ext == "" {
		ext = ".mp3"
	}

	savePath := fmt.Sprintf("uploads/%s%s", fileID, ext)

	if err := c.SaveUploadedFile(handler, savePath); err != nil {
		c.JSON(500, UploadResponse{
			Success:    false,
			StatusCode: 500,
			Message:    "Faylni saqlashda xatolik: " + err.Error(),
		})
		return
	}

	fileInfo, err := os.Stat(savePath)
	if err != nil {
		c.JSON(500, UploadResponse{
			Success:    false,
			StatusCode: 500,
			Message:    "Fayl ma'lumotlarini olishda xatolik: " + err.Error(),
		})
		return
	}
	fileSize := fileInfo.Size()

	audioURL := fmt.Sprintf("/static/%s%s", fileID, ext)

	contentType := "audio/mpeg"
	switch ext {
	case ".mp3":
		contentType = "audio/mpeg"
	case ".wav":
		contentType = "audio/wav"
	case ".ogg":
		contentType = "audio/ogg"
	case ".m4a":
		contentType = "audio/mp4"
	case ".aac":
		contentType = "audio/aac"
	case ".flac":
		contentType = "audio/flac"
	case ".wma":
		contentType = "audio/x-ms-wma"
	default:
		contentType = "audio/mpeg"
	}

	c.JSON(200, UploadResponse{
		Success:    true,
		StatusCode: 200,
		Message:    "Audio muvaffaqiyatli yuklandi",
		Data: UploadData{
			ID:          fileID,
			Size:        fileSize,
			URL:         audioURL,
			FileName:    handler.Filename,
			ContentType: contentType,
			DurationMs:  nil,
		},
	})
}

// @Summary Upload video
// @Description Upload a video file in any format
// @Tags uploads
// @Accept multipart/form-data
// @Produce json
// @Param video formData file true "Video file"
// @Success 200 {object} UploadResponse
// @Failure 400 {object} UploadResponse
// @Failure 500 {object} UploadResponse
// @Router /upload/video [post]
func uploadVideo(c *gin.Context) {
	file, handler, err := c.Request.FormFile("video")
	if err != nil {
		c.JSON(400, UploadResponse{
			Success:    false,
			StatusCode: 400,
			Message:    "Videoni olishda xatolik: " + err.Error(),
		})
		return
	}
	defer file.Close()

	fileID := uuid.New().String()
	ext := strings.ToLower(filepath.Ext(handler.Filename))

	if ext == "" {
		ext = ".mp4"
	}

	savePath := fmt.Sprintf("uploads/%s%s", fileID, ext)

	if err := c.SaveUploadedFile(handler, savePath); err != nil {
		c.JSON(500, UploadResponse{
			Success:    false,
			StatusCode: 500,
			Message:    "Faylni saqlashda xatolik: " + err.Error(),
		})
		return
	}

	fileInfo, err := os.Stat(savePath)
	if err != nil {
		c.JSON(500, UploadResponse{
			Success:    false,
			StatusCode: 500,
			Message:    "Fayl ma'lumotlarini olishda xatolik: " + err.Error(),
		})
		return
	}
	fileSize := fileInfo.Size()

	videoURL := fmt.Sprintf("/static/%s%s", fileID, ext)

	contentType := "video/mp4"
	switch ext {
	case ".mp4":
		contentType = "video/mp4"
	case ".mov":
		contentType = "video/quicktime"
	case ".avi":
		contentType = "video/x-msvideo"
	case ".webm":
		contentType = "video/webm"
	case ".mkv":
		contentType = "video/x-matroska"
	case ".flv":
		contentType = "video/x-flv"
	case ".wmv":
		contentType = "video/x-ms-wmv"
	case ".m4v":
		contentType = "video/x-m4v"
	default:
		contentType = "video/mp4"
	}

	c.JSON(200, UploadResponse{
		Success:    true,
		StatusCode: 200,
		Message:    "Video muvaffaqiyatli yuklandi",
		Data: UploadData{
			ID:          fileID,
			Size:        fileSize,
			URL:         videoURL,
			FileName:    handler.Filename,
			ContentType: contentType,
			DurationMs:  nil,
		},
	})
}
