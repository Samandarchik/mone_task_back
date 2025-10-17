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
	"github.com/gin-contrib/cors"
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

	_ "taskmanager/docs"
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
// @schemes http https

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
	// Environment variables with defaults
	dbHost := getEnv("DB_HOST", "localhost")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "password")
	dbName := getEnv("DB_NAME", "taskdb")
	dbPort := getEnv("DB_PORT", "5432")
	serverPort := getEnv("SERVER_PORT", "1212")
	baseURL := getEnv("BASE_URL", "http://localhost:1212")

	// Database connection
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Tashkent",
		dbHost, dbUser, dbPassword, dbName, dbPort)

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	// Auto migrate models
	if err := db.AutoMigrate(&Category{}, &Task{}, &TaskItem{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Create uploads directory
	if err := os.MkdirAll("uploads", os.ModePerm); err != nil {
		log.Fatal("Failed to create uploads directory:", err)
	}

	// Gin setup
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	// Static files
	r.Static("/uploads", "./uploads")
	r.Static("/static", "./uploads")

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "timestamp": time.Now()})
	})

	// Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API routes
	api := r.Group("/api")
	{
		// Categories
		api.POST("/categories", createCategory)
		api.GET("/categories", getCategories)
		api.GET("/categories/:id", getCategory)
		api.PUT("/categories/:id", updateCategory)
		api.DELETE("/categories/:id", deleteCategory)

		// Tasks
		api.POST("/tasks", createTask)
		api.GET("/tasks", getTasks)
		api.GET("/tasks/deleted", getDeletedTasks)
		api.GET("/tasks/:id", getTask)
		api.PUT("/tasks/:id", updateTask)
		api.PUT("/tasks/:id/position", updateTaskPosition)
		api.DELETE("/tasks/:id", deleteTask)
		api.PUT("/tasks/:id/restore", restoreTask)
		api.DELETE("/tasks/:id/permanent", permanentDeleteTask)
		api.PUT("/tasks/:id/success", markTaskSuccess)

		// Task Items
		api.POST("/task-items", createTaskItem)
		api.GET("/task-items", getTaskItems)
		api.GET("/task-items/:id", getTaskItem)
		api.GET("/tasks/:id/items", getTaskItemsByTaskID)
		api.PUT("/task-items/:id", updateTaskItem)
		api.DELETE("/task-items/:id", deleteTaskItem)

		// Uploads
		api.POST("/upload/image", uploadImage)
		api.POST("/upload/audio", uploadAudio)
		api.POST("/upload/video", uploadVideo)
	}

	// Root redirects
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/swagger/index.html")
	})

	log.Printf("Server starting on port %s", serverPort)
	log.Printf("Base URL: %s", baseURL)
	log.Printf("Swagger docs: %s/swagger/index.html", baseURL)
	log.Printf("Health check: %s/health", baseURL)

	if err := r.Run(":" + serverPort); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

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

func decodeImage(file multipart.File, ext string) (image.Image, string, error) {
	file.Seek(0, 0)

	if ext == ".heic" || ext == ".heif" {
		img, err := goheif.Decode(file)
		if err != nil {
			return nil, "", fmt.Errorf("HEIC decode error: %v", err)
		}
		return img, "heic", nil
	}

	if ext == ".webp" {
		img, err := webp.Decode(file)
		if err == nil {
			return img, "webp", nil
		}
	}

	if ext == ".bmp" {
		img, err := bmp.Decode(file)
		if err == nil {
			return img, "bmp", nil
		}
	}

	if ext == ".tiff" || ext == ".tif" {
		img, err := tiff.Decode(file)
		if err == nil {
			return img, "tiff", nil
		}
	}

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

// Category handlers
func createCategory(c *gin.Context) {
	var category Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	db.Create(&category)
	c.JSON(201, category)
}

func getCategories(c *gin.Context) {
	var categories []Category
	db.Find(&categories)
	c.JSON(200, categories)
}

func getCategory(c *gin.Context) {
	id := c.Param("id")
	var category Category
	if err := db.First(&category, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Category not found"})
		return
	}
	c.JSON(200, category)
}

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

func deleteCategory(c *gin.Context) {
	id := c.Param("id")
	if err := db.Delete(&Category{}, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Category not found"})
		return
	}
	c.JSON(200, gin.H{"message": "Category deleted"})
}

// Task handlers
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

func getTasks(c *gin.Context) {
	var tasks []Task
	db.Preload("Category").Preload("Items").Order("position ASC").Find(&tasks)

	var responses []TaskResponse
	for _, task := range tasks {
		responses = append(responses, convertToTaskResponse(task))
	}
	c.JSON(200, responses)
}

func getDeletedTasks(c *gin.Context) {
	var tasks []Task
	db.Unscoped().Preload("Category").Preload("Items").Where("deleted_at IS NOT NULL").Order("position ASC").Find(&tasks)

	var responses []TaskResponse
	for _, task := range tasks {
		responses = append(responses, convertToTaskResponse(task))
	}
	c.JSON(200, responses)
}

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

	if oldPos != newPos {
		if oldPos < newPos {
			db.Model(&Task{}).Where("position > ? AND position <= ?", oldPos, newPos).Update("position", gorm.Expr("position - 1"))
		} else {
			db.Model(&Task{}).Where("position >= ? AND position < ?", newPos, oldPos).Update("position", gorm.Expr("position + 1"))
		}
		task.Position = newPos
		db.Save(&task)
	}

	db.Preload("Category").Preload("Items").First(&task, task.ID)
	response := convertToTaskResponse(task)
	c.JSON(200, response)
}

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
			oldPath = strings.TrimPrefix(oldPath, "/uploads/")
			os.Remove(filepath.Join("uploads", oldPath))
		}
	}

	db.Unscoped().Where("task_id = ?", id).Delete(&TaskItem{})
	db.Unscoped().Delete(&task)
	c.JSON(200, gin.H{"message": "Task permanently deleted"})
}

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

func getTaskItems(c *gin.Context) {
	var items []TaskItem
	db.Find(&items)
	c.JSON(200, items)
}

func getTaskItem(c *gin.Context) {
	id := c.Param("id")
	var item TaskItem
	if err := db.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task item not found"})
		return
	}
	c.JSON(200, item)
}

func getTaskItemsByTaskID(c *gin.Context) {
	taskID := c.Param("id")
	var items []TaskItem
	db.Where("task_id = ?", taskID).Find(&items)
	c.JSON(200, items)
}

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

func deleteTaskItem(c *gin.Context) {
	id := c.Param("id")
	var item TaskItem
	if err := db.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Task item not found"})
		return
	}

	if item.Data != "" {
		oldPath := strings.TrimPrefix(item.Data, "/static/")
		oldPath = strings.TrimPrefix(oldPath, "/uploads/")
		os.Remove(filepath.Join("uploads", oldPath))
	}

	db.Delete(&item)
	c.JSON(200, gin.H{"message": "Task item deleted"})
}

// Upload handlers
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
	img, _, err := decodeImage(file, originalExt)
	if err != nil {
		c.JSON(400, UploadResponse{
			Success:    false,
			StatusCode: 400,
			Message:    "Rasmni o'qishda xatolik: " + err.Error(),
		})
		return
	}

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
	default:
		saveExt = ".jpg"
		contentType = "image/jpeg"
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	if width > 2048 {
		img = resize.Resize(2048, 0, img, resize.Lanczos3)
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

	fileInfo, _ := os.Stat(savePath)
	fileSize := fileInfo.Size()
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

	fileInfo, _ := os.Stat(savePath)
	fileSize := fileInfo.Size()
	audioURL := fmt.Sprintf("/uploads/%s%s", fileID, ext)

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

	fileInfo, _ := os.Stat(savePath)
	fileSize := fileInfo.Size()
	videoURL := fmt.Sprintf("/uploads/%s%s", fileID, ext)

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
