package router

import (
	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/controllers"
	"github.com/hock1024always/GoEdu/pkg/cors"
	"github.com/hock1024always/GoEdu/pkg/logger"
)

// 路由 函数的名字要大写，这样才可以被其他包访问！
func Router() *gin.Engine {
	//创建一个路由的实例
	r := gin.Default()

	//使用跨域中间件
	r.Use(cors.Cors())

	//日志中间件
	r.Use(gin.LoggerWithConfig(logger.LoggerToFile()))
	r.Use(logger.Recover)
	// 设置最大上传文件大小为10MB
	r.MaxMultipartMemory = 10 << 20

	////sessions中间件
	//store, _ := sessions_redis.NewStore(10, "tcp", config.RedisAddress, "", []byte("secret"))
	//r.Use(sessions.Sessions("mysession", store))

	user := r.Group("/user")
	{
		// 向注册用户发送验证码 username password confirm_password email
		user.POST("/register", controllers.UserController{}.Register)
		// 实现注册用户验证码 username password email confirm_code
		user.POST("/register/verify", controllers.UserController{}.Verify)
		// 登录用户相关的路由 username password
		user.POST("/login", controllers.UserController{}.Login)
		////实现删除用户的路由 token
		//user.POST("/delete", controllers.UserController{}.UserDelete)
		////实现websocket的路由 token
		//user.POST("/ws", controllers.UserController{}.WsHandler)
		////实现用户视频功能
		//user.GET("/video", controllers.UserController{}.LiveHandler)
		//// 实现用户获取自己的聊天记录 token
		//user.POST("/get_chat_records", controllers.UserController{}.GetChatRecords)

	}

	ppt := r.Group("/ppt")
	{
		// 获取ppt大纲 string  返json
		ppt.POST("/review", controllers.PptController{}.PptReview)
		// 获取ppt资源 string  返ppt
		ppt.POST("/resource", controllers.PptController{}.PptResource)

	}

	ai := r.Group("/ai")
	{
		//实现用户与ai的聊天
		ai.GET("/ws", controllers.AIController{}.AIHandler)
		//上传视频获取分析报告 string
		ai.POST("/upload", controllers.AIController{}.Upload)
		//获取优化报告 string
		ai.POST("/optimize", controllers.AIController{}.Optimize)
		////在线课堂
		//ai.GET("/class", controllers.AIController{}.Class)
		////多模态分析 string
		//ai.POST("/multimodal", controllers.AIController{}.Multimodal)
		//数字人
		ai.GET("/digital/:name", controllers.AIController{}.Digital)
		//资源列表
		ai.GET("/video/list", controllers.AIController{}.VideoList)
	}

	static := r.Group("/static")
	{
		//获取思维导图 string
		static.POST("/mindmap", controllers.StaticController{}.MindMap)
		//获取试卷 string
		static.POST("/exam", controllers.StaticController{}.Exam)
		//获取名师教案 string
		static.POST("/teach", controllers.StaticController{}.Teach)
		//获取名师教案 string
		static.POST("/anything", controllers.StaticController{}.Anything)
	}

	student := r.Group("/student")
	{
		//学生登录
		student.POST("/login", controllers.StudentController{}.Login)
		// 实现注册用户验证码 username password email confirm_code
		student.POST("/register/verify", controllers.StudentController{}.Verify)
		//学生注册
		student.POST("/register", controllers.StudentController{}.Register)

		//上传论文
		student.POST("/upload", controllers.StudentController{}.Upload)
		//学生上传答辩材料
		student.POST("/upload_answer", controllers.StudentController{}.UploadAnswer)
		//学生获取总成绩
		student.GET("/get_score", controllers.StudentController{}.GetScore)
	}

	// 对话助手接口（新增）
	dialogCtrl := controllers.NewDialogController()
	dialog := r.Group("/api/dialog")
	{
		// SSE 流式对话
		dialog.POST("/chat", dialogCtrl.Chat)
		// 获取会话历史
		dialog.GET("/history", dialogCtrl.GetHistory)
		// 获取会话列表
		dialog.GET("/sessions", dialogCtrl.ListSessions)
		// 知识库搜索
		dialog.POST("/search", dialogCtrl.SearchKnowledge)
		// Agent 统计指标
		dialog.GET("/agent/stats", dialogCtrl.GetAgentStats)
	}

	// 前端静态文件服务
	r.Static("/app", "../frontend")

	// 备课工作流接口
	lessonPrepCtrl := controllers.NewLessonPrepController()
	lessonPrep := r.Group("/api/lessonprep")
	{
		// 启动工作流：输入需求 → 生成大纲（SSE）
		lessonPrep.POST("/start", lessonPrepCtrl.Start)
		// 对话式修改大纲（SSE）
		lessonPrep.POST("/outline/edit", lessonPrepCtrl.EditOutline)
		// 确认大纲
		lessonPrep.POST("/outline/confirm", lessonPrepCtrl.ConfirmOutline)
		// 获取大纲详情
		lessonPrep.GET("/outline", lessonPrepCtrl.GetOutline)
		// 生成教案（SSE）
		lessonPrep.POST("/plan/generate", lessonPrepCtrl.GeneratePlan)
		// 获取教案详情
		lessonPrep.GET("/plan", lessonPrepCtrl.GetPlan)
		// 生成 PPT（SSE）
		lessonPrep.POST("/ppt/generate", lessonPrepCtrl.GeneratePPT)
		// 下载 PPT
		lessonPrep.GET("/ppt/download", lessonPrepCtrl.DownloadPPT)
		// 获取工作流状态
		lessonPrep.GET("/status", lessonPrepCtrl.GetStatus)
		// 列出工作流
		lessonPrep.GET("/list", lessonPrepCtrl.ListWorkflows)
	}

	// 数字人视频生成接口
	dhCtrl := controllers.NewDigitalHumanController()
	dh := r.Group("/api/digital-human")
	{
		// 提交生成任务
		dh.POST("/videos", dhCtrl.CreateVideo)
		// 查询任务状态
		dh.GET("/videos/:id", dhCtrl.GetVideo)
		// 任务列表
		dh.GET("/videos", dhCtrl.ListVideos)
		// 手动重试
		dh.POST("/videos/:id/retry", dhCtrl.RetryVideo)
		// 取消任务
		dh.DELETE("/videos/:id", dhCtrl.CancelVideo)
		// Webhook 回调（供外部 API 推送状态）
		dh.POST("/webhook", dhCtrl.Webhook)
		// 获取可用数字人形象列表
		dh.GET("/avatars", dhCtrl.ListAvatars)
	}

	// 缓存管理接口
	cacheCtrl := controllers.NewCacheController()
	cacheGroup := r.Group("/api/cache")
	{
		// 获取缓存统计
		cacheGroup.GET("/stats", cacheCtrl.GetStats)
		// 清除缓存
		cacheGroup.DELETE("", cacheCtrl.ClearCache)
		// 重置统计
		cacheGroup.POST("/stats/reset", cacheCtrl.ResetStats)
		// 缓存状态
		cacheGroup.GET("/status", cacheCtrl.CacheStatus)
	}

	return r
}
