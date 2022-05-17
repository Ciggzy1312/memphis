// Copyright 2021-2022 The Memphis Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routes

import (
	"memphis-control-plane/handlers"

	"github.com/gin-gonic/gin"
)

func InitializeUserMgmtRoutes(router *gin.RouterGroup) {
	userMgmtHandler := handlers.UserMgmtHandler{}
	userMgmtRoutes := router.Group("/usermgmt")
	userMgmtRoutes.GET("/nats/authenticate", userMgmtHandler.AuthenticateNatsUser)
	userMgmtRoutes.GET("/nats/authenticate/:publicKey", userMgmtHandler.AuthenticateNatsUser)
	userMgmtRoutes.POST("/login", userMgmtHandler.Login)
	userMgmtRoutes.POST("/refreshToken", userMgmtHandler.RefreshToken)
	userMgmtRoutes.POST("/logout", userMgmtHandler.Logout)
	userMgmtRoutes.POST("/addUser", userMgmtHandler.AddUser)
	userMgmtRoutes.GET("/getAllUsers", userMgmtHandler.GetAllUsers)
	userMgmtRoutes.DELETE("/removeUser", userMgmtHandler.RemoveUser)
	userMgmtRoutes.DELETE("/removeMyUser", userMgmtHandler.RemoveMyUser)
	userMgmtRoutes.PUT("/editAvatar", userMgmtHandler.EditAvatar)
	userMgmtRoutes.PUT("/editHubCreds", userMgmtHandler.EditHubCreds)
	userMgmtRoutes.PUT("/editCompanyLogo", userMgmtHandler.EditCompanyLogo)
	userMgmtRoutes.DELETE("/removeCompanyLogo", userMgmtHandler.RemoveCompanyLogo)
	userMgmtRoutes.GET("/getCompanyLogo", userMgmtHandler.GetCompanyLogo)
	userMgmtRoutes.PUT("/editAnalytics", userMgmtHandler.EditAnalytics)
}