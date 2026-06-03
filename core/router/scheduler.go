package router

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/config"
	plugincore "github.com/allbot/allbot/core/plugin"
)

type ScheduledTaskRunner struct {
	database *config.Database
	router   *Router
	stop     chan struct{}
	once     sync.Once
}

func NewScheduledTaskRunner(database *config.Database, router *Router) *ScheduledTaskRunner {
	return &ScheduledTaskRunner{database: database, router: router, stop: make(chan struct{})}
}

func (r *ScheduledTaskRunner) Start() {
	if r == nil || r.database == nil || r.router == nil {
		return
	}
	go r.loop()
}

func (r *ScheduledTaskRunner) Stop() {
	if r == nil {
		return
	}
	r.once.Do(func() { close(r.stop) })
}

func (r *ScheduledTaskRunner) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	if err := r.prepareStartupNextRun(time.Now()); err != nil {
		log.Printf("[SYSTEM] 初始化启动定时任务下次执行时间失败: %v", err)
	}
	r.tick()
	for {
		select {
		case <-ticker.C:
			r.tick()
		case <-r.stop:
			return
		}
	}
}

func (r *ScheduledTaskRunner) prepareStartupNextRun(now time.Time) error {
	tasks, err := r.database.ListScheduledTasks()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if !task.Enabled || isOnceCron(task.Cron) {
			continue
		}
		if task.NextRunAt != nil && task.NextRunAt.After(now) {
			continue
		}
		next, err := NextCronTime(task.Cron, now)
		if err != nil {
			log.Printf("[SYSTEM] 定时任务 %d 表达式无效: %v", task.ID, err)
			continue
		}
		if err := r.database.UpdateScheduledTaskNextRun(task.ID, &next); err != nil {
			return err
		}
	}
	return nil
}

func (r *ScheduledTaskRunner) tick() {
	now := time.Now()
	if err := r.initializeNextRun(now); err != nil {
		log.Printf("[SYSTEM] 初始化定时任务下次执行时间失败: %v", err)
	}
	tasks, err := r.database.ListDueScheduledTasks(now)
	if err != nil {
		log.Printf("[SYSTEM] 加载到期定时任务失败: %v", err)
		return
	}
	for _, task := range tasks {
		r.runTask(task, now)
	}
}

func (r *ScheduledTaskRunner) initializeNextRun(now time.Time) error {
	tasks, err := r.database.ListScheduledTasks()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if !task.Enabled || isOnceCron(task.Cron) || task.NextRunAt != nil {
			continue
		}
		next, err := NextCronTime(task.Cron, now)
		if err != nil {
			log.Printf("[SYSTEM] 定时任务 %d 表达式无效: %v", task.ID, err)
			continue
		}
		if err := r.database.UpdateScheduledTaskNextRun(task.ID, &next); err != nil {
			return err
		}
	}
	return nil
}

func (r *ScheduledTaskRunner) runTask(task *config.ScheduledTask, now time.Time) {
	next, err := NextCronTime(task.Cron, now)
	if err != nil {
		log.Printf("[SYSTEM] 定时任务 %d 表达式无效: %v", task.ID, err)
		_ = r.database.MarkScheduledTaskRun(task.ID, now, nil)
		return
	}
	pluginID := strings.TrimSpace(task.PluginID)
	if pluginID == "" {
		pluginID = fmt.Sprintf("scheduled-task-%d", task.ID)
	}
	log.Printf("[SYSTEM] 执行定时伪造消息: id=%d plugin=%s platform=%s adapter_id=%s user=%s group=%s content=%s", task.ID, task.PluginID, task.Platform, task.AdapterID, task.UserID, task.GroupID, task.Content)
	if err := r.router.dispatchFakeMessage(pluginID, plugincore.FakeMessageAction{Platform: task.Platform, AdapterID: task.AdapterID, UserID: task.UserID, GroupID: task.GroupID, Content: task.Content}); err != nil {
		log.Printf("[SYSTEM] 定时伪造消息失败: id=%d err=%v", task.ID, err)
	}
	if err := r.database.MarkScheduledTaskRun(task.ID, now, &next); err != nil {
		log.Printf("[SYSTEM] 更新定时任务执行时间失败: id=%d err=%v", task.ID, err)
	}
}

func NextCronTime(expression string, after time.Time) (time.Time, error) {
	schedules, err := parseCronExpression(expression)
	if err != nil {
		return time.Time{}, err
	}
	if len(schedules) == 0 {
		return time.Time{}, fmt.Errorf("@once 任务只能手动启动")
	}
	candidate := after.Add(time.Second).Truncate(time.Second)
	deadline := after.AddDate(1, 0, 0)
	for !candidate.After(deadline) {
		for _, schedule := range schedules {
			if schedule.matches(candidate) {
				return candidate, nil
			}
		}
		candidate = candidate.Add(time.Second)
	}
	return time.Time{}, fmt.Errorf("未来一年内没有匹配时间")
}

type cronSchedule struct {
	seconds map[int]bool
	minutes map[int]bool
	hours   map[int]bool
	days    map[int]bool
	months  map[int]bool
	weeks   map[int]bool
}

func (s cronSchedule) matches(t time.Time) bool {
	return s.seconds[t.Second()] && s.minutes[t.Minute()] && s.hours[t.Hour()] && s.days[t.Day()] && s.months[int(t.Month())] && s.weeks[int(t.Weekday())]
}

func parseCronExpression(expression string) ([]cronSchedule, error) {
	expressions := splitCronExpressions(expression)
	if len(expressions) == 0 {
		return nil, fmt.Errorf("定时表达式不能为空")
	}
	if len(expressions) == 1 && strings.EqualFold(expressions[0], "@once") {
		return nil, nil
	}

	schedules := make([]cronSchedule, 0, len(expressions))
	for index, item := range expressions {
		if strings.EqualFold(item, "@once") {
			return nil, fmt.Errorf("@once 不能和其他定时表达式混用")
		}
		schedule, err := parseSingleCronExpression(item)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个表达式错误: %w", index+1, err)
		}
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func splitCronExpressions(expression string) []string {
	normalized := strings.ReplaceAll(strings.TrimSpace(expression), "\r\n", "\n")
	items := make([]string, 0)
	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		items = append(items, line)
	}
	return items
}

func parseSingleCronExpression(expression string) (cronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expression))
	if len(fields) == 5 {
		fields = append([]string{"0"}, fields...)
	}
	if len(fields) != 6 {
		return cronSchedule{}, fmt.Errorf("定时表达式需要 5 位或 6 位")
	}
	seconds, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("秒字段错误: %w", err)
	}
	minutes, err := parseCronField(fields[1], 0, 59)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("分钟字段错误: %w", err)
	}
	hours, err := parseCronField(fields[2], 0, 23)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("小时字段错误: %w", err)
	}
	days, err := parseCronField(fields[3], 1, 31)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("日期字段错误: %w", err)
	}
	months, err := parseCronField(fields[4], 1, 12)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("月份字段错误: %w", err)
	}
	weeks, err := parseCronField(strings.ReplaceAll(fields[5], "7", "0"), 0, 6)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("星期字段错误: %w", err)
	}
	return cronSchedule{seconds: seconds, minutes: minutes, hours: hours, days: days, months: months, weeks: weeks}, nil
}

func parseCronField(field string, minValue int, maxValue int) (map[int]bool, error) {
	result := make(map[int]bool)
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("存在空白片段")
		}
		base := part
		step := 1
		if strings.Contains(part, "/") {
			pieces := strings.Split(part, "/")
			if len(pieces) != 2 {
				return nil, fmt.Errorf("步进格式错误: %s", part)
			}
			base = pieces[0]
			parsedStep, err := strconv.Atoi(pieces[1])
			if err != nil || parsedStep <= 0 {
				return nil, fmt.Errorf("步进值错误: %s", part)
			}
			step = parsedStep
		}
		start, end, err := parseCronRange(base, minValue, maxValue)
		if err != nil {
			return nil, err
		}
		for value := start; value <= end; value += step {
			result[value] = true
		}
	}
	return result, nil
}

func parseCronRange(value string, minValue int, maxValue int) (int, int, error) {
	if value == "*" || value == "" {
		return minValue, maxValue, nil
	}
	if strings.Contains(value, "-") {
		pieces := strings.Split(value, "-")
		if len(pieces) != 2 {
			return 0, 0, fmt.Errorf("范围格式错误: %s", value)
		}
		start, err := parseCronNumber(pieces[0], minValue, maxValue)
		if err != nil {
			return 0, 0, err
		}
		end, err := parseCronNumber(pieces[1], minValue, maxValue)
		if err != nil {
			return 0, 0, err
		}
		if start > end {
			return 0, 0, fmt.Errorf("范围起点不能大于终点: %s", value)
		}
		return start, end, nil
	}
	number, err := parseCronNumber(value, minValue, maxValue)
	return number, number, err
}

func isOnceCron(expression string) bool {
	return strings.EqualFold(strings.TrimSpace(expression), "@once")
}

func parseCronNumber(value string, minValue int, maxValue int) (int, error) {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("不是数字: %s", value)
	}
	if number < minValue || number > maxValue {
		return 0, fmt.Errorf("超出范围 %d-%d: %d", minValue, maxValue, number)
	}
	return number, nil
}
