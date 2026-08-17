package main

import (
	"context"
	"fmt"
	"os"

	"evidence/internal/application"
	"evidence/internal/repository"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--self-check" {
		runSelfCheck()
		return
	}
	fmt.Println("授权证据生命周期核心已启动，使用 --self-check 执行自检")
}

func runSelfCheck() {
	ctx := context.Background()
	repo, err := repository.NewFileRepository("./selfcheck-data")
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化仓库失败: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll("./selfcheck-data")
	svc := application.NewAuthorizationService(repo)
	if err := svc.SelfCheck(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "自检失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("self-check passed")
	os.Exit(0)
}
