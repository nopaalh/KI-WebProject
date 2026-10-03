package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	PORT := ":3001"

	r.Run(PORT)

}
