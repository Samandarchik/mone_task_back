package main

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrium/goheif"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfnt/resize"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "taskmanager/docs" // Swagger docs import
)

// @title Task Management API
// @version 1.0
// @description Task Management API with Categories, Tasks and Task Items
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
	ID   uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Data string    `json:"data" example:"Work"`
}

type Task struct {
	ID         uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id" example:"550e8400-e29b-41d4-a716-446655440001"`
	CategoryID uuid.UUID      `gorm:"type:uuid" json:"category_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name       string         `json:"name" example:"Complete project"`
	IsSuccess  bool           `gorm:"default:false" json:"is_success" example:"false"`
	Price      *float32       `json:"price" example:"100.50"`
	Position   int            `json:"position" example:"1"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	Category   Category       `gorm:"foreignKey:CategoryID" json:"category"`
	Items      []TaskItem     `gorm:"foreignKey:TaskID" json:"items"`
}

type TaskItem struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id" example:"550e8400-e29b-41d4-a716-446655440002"`
	TaskID uuid.UUID `gorm:"type:uuid" json:"task_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Type   string    `json:"type" example:"image"`
	Data   string    `json:"data" example:"/static/image.jpg"`
	Time   time.Time `json:"time" example:"2025-10-17T12:00:00Z"`
}

// Response structures
type TaskItemResponse struct {
	ID   uuid.UUID `json:"id"`
	Type string    `json:"type"`
	Data struct {
		ID   uuid.UUID `json:"id"`
		Data string    `json:"data"`
		Time time.Time `json:"time"`
	} `json:"data"`
}

type TaskResponse struct {
	ID         uuid.UUID          `json:"id"`
	CategoryID uuid.UUID          `json:"category_id"`
	Name       string             `json:"name"`
	IsSuccess  bool               `json:"is_success"`
	Price      *float32           `json:"price"`
	Position   int                `json:"position"`
	DeletedAt  *time.Time         `json:"deleted_at,omitempty"`
	Category   []Category         `json:"category"`
	TaskName   []TaskItemResponse `json:"task_name"`
}

// Upload response structures
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

var db *gorm.DB

func main() {
	dsn := "host=localhost user=postgres password=password dbname=taskdb port=5432 sslmode=disable"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&Category{}, &Task{}, &TaskItem{})

	r := gin.Default()
	r.Static("/static", "./uploads")
	os.MkdirAll("uploads", os.ModePerm)

	// Swagger documentation route
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

	log.Println("Server running on :1212")
	log.Println("Swagger documentation available at http://localhost:1212/swagger/index.html")
	r.Run(":1212")
}

// Helper function to convert Task to TaskResponse
func convertToTaskResponse(task Task) TaskResponse {
	response := TaskResponse{
		ID:         task.ID,
		CategoryID: task.CategoryID,
		Name:       task.Name,
		IsSuccess:  task.IsSuccess,
		Price:      task.Price,
		Position:   task.Position,
		Category:   []Category{task.Category},
		TaskName:   []TaskItemResponse{},
	}

	if !task.DeletedAt.Time.IsZero() {
		deletedTime := task.DeletedAt.Time
		response.DeletedAt = &deletedTime
	}

	for _, item := range task.Items {
		itemResp := TaskItemResponse{
			ID:   item.ID,
			Type: item.Type,
		}
		itemResp.Data.ID = item.ID
		itemResp.Data.Data = item.Data
		itemResp.Data.Time = item.Time
		response.TaskName = append(response.TaskName, itemResp)
	}

	return response
}

// Helper function to decode any image format
func decodeImage(file multipart.File, ext string) (image.Image, string, error) {
	file.Seek(0, 0)

	// Try HEIC/HEIF first
	if ext == ".heic" || ext == ".heif" {
		img, err := goheif.Decode(file)
		if err != nil {
			return nil, "", fmt.Errorf("HEIC decode error: %v", err)
		}
		return img, "heic", nil
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

// Helper function to save image in original format or convert to JPEG
func saveImage(img image.Image, savePath string, originalExt string) error {
	out, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("file creation error: %v", err)
	}
	defer out.Close()

	// Determine save format based on extension
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
		// Default to JPEG for all other formats (including HEIC, HEIF, WebP)
		opts := &jpeg.Options{Quality: 90}
		return jpeg.Encode(out, img, opts)
	}
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
	db.Create(&category)
	c.JSON(201, category)
}

// @Summary Get all categories
// @Description Get a list of all categories
// @Tags categories
// @Produce json
// @Success 200 {array} Category
// @Router /categories [get]
func getCategories(c *gin.Context) {
	var categories []Category
	db.Find(&categories)
	c.JSON(200, categories)
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
	var category Category
	if err := db.First(&category, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Category not found"})
		return
	}
	c.JSON(200, category)
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
	var category Category
	if err := db.First(&category, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Category not found"})
		return
	}
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	db.Save(&category)
	c.JSON(200, category)
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
	if err := db.Delete(&Category{}, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Category not found"})
		return
	}
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
		CategoryID uuid.UUID `json:"category_id"`
		Name       string    `json:"name"`
		IsSuccess  bool      `json:"is_success"`
		Price      *float32  `json:"price"`
		Position   *int      `json:"position"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var task Task
	task.CategoryID = input.CategoryID
	task.Name = input.Name
	task.IsSuccess = input.IsSuccess
	task.Price = input.Price

	if input.Position == nil {
		var maxPos int
		db.Model(&Task{}).Select("COALESCE(MAX(position), -1)").Scan(&maxPos)
		task.Position = maxPos + 1
	} else {
		task.Position = *input.Position
		db.Model(&Task{}).Where("position >= ?", *input.Position).Update("position", gorm.Expr("position + 1"))
	}

	db.Create(&task)

	db.Preload("Category").Preload("Items").First(&task, task.ID)
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
	var tasks []Task
	db.Preload("Category").Preload("Items").Order("position ASC").Find(&tasks)

	var responses []TaskResponse
	for _, task := range tasks {
		responses = append(responses, convertToTaskResponse(task))
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
	var tasks []Task
	db.Unscoped().Preload("Category").Preload("Items").Where("deleted_at IS NOT NULL").Order("position ASC").Find(&tasks)

	var responses []TaskResponse
	for _, task := range tasks {
		responses = append(responses, convertToTaskResponse(task))
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
	var task Task
	if err := db.Preload("Category").Preload("Items").First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	response := convertToTaskResponse(task)
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
	var task Task
	if err := db.First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	var input struct {
		CategoryID uuid.UUID `json:"category_id"`
		Name       string    `json:"name"`
		IsSuccess  bool      `json:"is_success"`
		Price      *float32  `json:"price"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	task.CategoryID = input.CategoryID
	task.Name = input.Name
	task.IsSuccess = input.IsSuccess
	task.Price = input.Price

	db.Save(&task)

	db.Preload("Category").Preload("Items").First(&task, task.ID)
	response := convertToTaskResponse(task)

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
	var task Task
	if err := db.First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	var input struct {
		Position int `json:"position"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	oldPos := task.Position
	newPos := input.Position

	if oldPos == newPos {
		db.Preload("Category").Preload("Items").First(&task, task.ID)
		response := convertToTaskResponse(task)
		c.JSON(200, response)
		return
	}

	if oldPos < newPos {
		db.Model(&Task{}).Where("position > ? AND position <= ?", oldPos, newPos).Update("position", gorm.Expr("position - 1"))
	} else {
		db.Model(&Task{}).Where("position >= ? AND position < ?", newPos, oldPos).Update("position", gorm.Expr("position + 1"))
	}

	task.Position = newPos
	db.Save(&task)

	db.Preload("Category").Preload("Items").First(&task, task.ID)
	response := convertToTaskResponse(task)

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
	var task Task
	if err := db.First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	db.Delete(&task)
	db.Model(&Task{}).Where("position > ?", task.Position).Update("position", gorm.Expr("position - 1"))

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
	var task Task
	if err := db.Unscoped().First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	if task.DeletedAt.Time.IsZero() {
		c.JSON(400, gin.H{"error": "Task is not deleted"})
		return
	}

	var maxPos int
	db.Model(&Task{}).Select("COALESCE(MAX(position), -1)").Scan(&maxPos)
	task.Position = maxPos + 1

	db.Unscoped().Model(&task).Update("deleted_at", nil)
	db.Save(&task)

	db.Preload("Category").Preload("Items").First(&task, task.ID)
	response := convertToTaskResponse(task)

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
	var task Task
	if err := db.Unscoped().First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	var items []TaskItem
	db.Where("task_id = ?", id).Find(&items)
	for _, item := range items {
		if item.Data != "" {
			oldPath := strings.TrimPrefix(item.Data, "/static/")
			os.Remove(filepath.Join("uploads", oldPath))
		}
	}
	db.Unscoped().Where("task_id = ?", id).Delete(&TaskItem{})
	db.Unscoped().Delete(&task)

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
	var task Task
	if err := db.First(&task, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}

	var input struct {
		IsSuccess bool     `json:"is_success"`
		Price     *float32 `json:"price"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	task.IsSuccess = input.IsSuccess
	task.Price = input.Price

	db.Save(&task)

	db.Preload("Category").Preload("Items").First(&task, task.ID)
	response := convertToTaskResponse(task)

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
	item.Time = time.Now()
	db.Create(&item)
	c.JSON(201, item)
}

// @Summary Get all task items
// @Description Get a list of all task items
// @Tags task-items
// @Produce json
// @Success 200 {array} TaskItem
// @Router /task-items [get]
func getTaskItems(c *gin.Context) {
	var items []TaskItem
	db.Find(&items)
	c.JSON(200, items)
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
	var item TaskItem
	if err := db.First(&item, "id = ?", id).Error; err != nil {
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
	var items []TaskItem
	db.Where("task_id = ?", taskID).Find(&items)
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
	var item TaskItem
	if err := db.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task item not found"})
		return
	}
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	db.Save(&item)
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
	var item TaskItem
	if err := db.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task item not found"})
		return
	}

	if item.Data != "" {
		oldPath := strings.TrimPrefix(item.Data, "/static/")
		os.Remove(filepath.Join("uploads", oldPath))
	}

	db.Delete(&item)
	c.JSON(200, gin.H{"message": "Task item deleted"})
}

// File upload handlers

// @Summary Upload image
// @Description Upload an image file in any format (JPEG, PNG, GIF, BMP, TIFF, WebP, HEIC, HEIF)
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

	// Get original extension
	originalExt := strings.ToLower(filepath.Ext(handler.Filename))

	// Decode image
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

	// Generate unique ID
	fileID := uuid.New().String()

	// Determine save extension (keep original or convert to JPEG)
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
		saveExt = ".jpg" // Convert WebP to JPEG
		contentType = "image/jpeg"
	case ".heic", ".heif":
		saveExt = ".jpg" // Convert HEIC to JPEG
		contentType = "image/jpeg"
	default:
		saveExt = ".jpg" // Default to JPEG
		contentType = "image/jpeg"
	}

	// Resize image (optional - only if image is too large)
	bounds := img.Bounds()
	width := bounds.Dx()
	if width > 2048 {
		img = resize.Resize(2048, 0, img, resize.Lanczos3)
		log.Println("Image resized to 2048px width")
	}

	// Save path
	savePath := fmt.Sprintf("uploads/%s%s", fileID, saveExt)

	// Save image
	err = saveImage(img, savePath, saveExt)
	if err != nil {
		c.JSON(500, UploadResponse{
			Success:    false,
			StatusCode: 500,
			Message:    "Rasmni saqlashda xatolik: " + err.Error(),
		})
		return
	}

	// Get file size
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

	// Generate URL
	imageURL := fmt.Sprintf("/uploads/%s%s", fileID, saveExt)

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

	// Generate unique ID
	fileID := uuid.New().String()
	ext := strings.ToLower(filepath.Ext(handler.Filename))

	// If no extension, default to .mp3
	if ext == "" {
		ext = ".mp3"
	}

	savePath := fmt.Sprintf("uploads/%s%s", fileID, ext)

	// Save file
	if err := c.SaveUploadedFile(handler, savePath); err != nil {
		c.JSON(500, UploadResponse{
			Success:    false,
			StatusCode: 500,
			Message:    "Faylni saqlashda xatolik: " + err.Error(),
		})
		return
	}

	// Get file size
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

	audioURL := fmt.Sprintf("http://31.187.74.228:1212/test/uploads/%s%s", fileID, ext)

	// Determine content type based on extension
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

	// Generate unique ID
	fileID := uuid.New().String()
	ext := strings.ToLower(filepath.Ext(handler.Filename))

	// If no extension, default to .mp4
	if ext == "" {
		ext = ".mp4"
	}

	savePath := fmt.Sprintf("uploads/%s%s", fileID, ext)

	// Save file
	if err := c.SaveUploadedFile(handler, savePath); err != nil {
		c.JSON(500, UploadResponse{
			Success:    false,
			StatusCode: 500,
			Message:    "Faylni saqlashda xatolik: " + err.Error(),
		})
		return
	}

	// Get file size
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

	videoURL := fmt.Sprintf("http://31.187.74.228:1212/test/uploads/%s%s", fileID, ext)

	// Determine content type based on extension
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
