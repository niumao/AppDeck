package models

import "time"

type ShellLog struct {
	ID          string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	MachineCode string    `gorm:"column:machine_code;size:255;not null;index" json:"machine_code"`
	Command     string    `gorm:"column:command;type:text;not null" json:"command"`
	Stdout      string    `gorm:"column:stdout;type:mediumtext" json:"stdout"`
	Stderr      string    `gorm:"column:stderr;type:mediumtext" json:"stderr"`
	ExitCode    int       `gorm:"column:exit_code;default:-1" json:"exit_code"`
	Status      string    `gorm:"column:status;size:20;not null;default:'pending'" json:"status"`
	DurationMs  int       `gorm:"column:duration_ms;default:0" json:"duration_ms"`
	ExecutedBy  string    `gorm:"column:executed_by;size:100" json:"executed_by"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ShellLog) TableName() string {
	return "shell_logs"
}
