package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/ccfos/nightingale/v6/pkg/poster"
	"github.com/ccfos/nightingale/v6/pkg/tplx"
	"github.com/ccfos/nightingale/v6/pkg/unit"

	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
)

type AlertCurEvent struct {
	Id                 int64               `json:"id" gorm:"primaryKey"`
	Cate               string              `json:"cate"`
	Cluster            string              `json:"cluster"`
	DatasourceId       int64               `json:"datasource_id"`
	GroupId            int64               `json:"group_id"`   // busi group id
	GroupName          string              `json:"group_name"` // busi group name
	Hash               string              `json:"hash"`       // rule_id + vector_key
	RuleId             int64               `json:"rule_id"`
	RuleName           string              `json:"rule_name"`
	RuleNote           string              `json:"rule_note"`
	RuleProd           string              `json:"rule_prod"`
	RuleAlgo           string              `json:"rule_algo"`
	Severity           int                 `json:"severity"`
	PromForDuration    int                 `json:"prom_for_duration"`
	PromQl             string              `json:"prom_ql"`
	RuleConfig         string              `json:"-" gorm:"rule_config"` // rule config
	RuleConfigJson     interface{}         `json:"rule_config" gorm:"-"` // rule config for fe
	PromEvalInterval   int                 `json:"prom_eval_interval"`
	Callbacks          string              `json:"-"`                  // for db
	CallbacksJSON      []string            `json:"callbacks" gorm:"-"` // for fe
	RunbookUrl         string              `json:"runbook_url"`
	NotifyRecovered    int                 `json:"notify_recovered"`
	NotifyChannels     string              `json:"-"`                                    // for db
	NotifyChannelsJSON []string            `json:"notify_channels,omitempty" gorm:"-"`   // for fe
	NotifyGroups       string              `json:"-"`                                    // for db
	NotifyGroupsJSON   []string            `json:"notify_groups,omitempty" gorm:"-"`     // for fe
	NotifyGroupsObj    []*UserGroup        `json:"notify_groups_obj,omitempty" gorm:"-"` // for fe
	TargetIdent        string              `json:"target_ident"`
	TargetNote         string              `json:"target_note"`
	TriggerTime        int64               `json:"trigger_time"`
	TriggerValue       string              `json:"trigger_value"`
	TriggerValues      string              `json:"trigger_values" gorm:"-"`
	TriggerValuesJson  EventTriggerValues  `json:"trigger_values_json" gorm:"-"`
	Tags               string              `json:"-"`                                   // for db
	TagsJSON           []string            `json:"tags" gorm:"-"`                       // for fe
	TagsMap            map[string]string   `json:"tags_map" gorm:"-"`                   // for internal usage
	OriginalTags       string              `json:"-"`                                   // for db
	OriginalTagsJSON   []string            `json:"original_tags" gorm:"-"`              // for fe
	Annotations        string              `json:"-"`                                   //
	AnnotationsJSON    map[string]string   `json:"annotations" gorm:"-"`                // for fe
	IsRecovered        bool                `json:"is_recovered" gorm:"-"`               // for notify.py
	NotifyUsersObj     []*User             `json:"notify_users_obj,omitempty" gorm:"-"` // for notify.py
	LastEvalTime       int64               `json:"last_eval_time" gorm:"-"`             // for notify.py 上次计算的时间
	LastSentTime       int64               `json:"last_sent_time" gorm:"-"`             // 上次发送时间
	FirstEvalTime      int64               `json:"first_eval_time" gorm:"-"`            // 首次异常检测时间
	NotifyCurNumber    int                 `json:"notify_cur_number"`                   // notify: current number
	FirstTriggerTime   int64               `json:"first_trigger_time"`                  // 连续告警的首次告警时间
	ExtraConfig        interface{}         `json:"extra_config" gorm:"-"`
	Status             int                 `json:"status" gorm:"-"`
	Claimant           string              `json:"claimant" gorm:"-"`
	SubRuleId          int64               `json:"sub_rule_id" gorm:"-"`
	ExtraInfo          []string            `json:"extra_info" gorm:"-"`
	Target             *Target             `json:"target" gorm:"-"`
	RecoverConfig      RecoverConfig       `json:"recover_config" gorm:"-"`
	RuleHash           string              `json:"rule_hash" gorm:"-"`
	ExtraInfoMap       []map[string]string `json:"extra_info_map" gorm:"-"`
	NotifyRuleIds      []int64             `json:"notify_rule_ids" gorm:"serializer:json"`

	NotifyVersion int                `json:"notify_version"  gorm:"-"` // 0: old, 1: new
	NotifyRules   []*EventNotifyRule `json:"notify_rules" gorm:"-"`
}

type EventNotifyRule struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

func (e *AlertCurEvent) SetTagsMap() {
	e.TagsMap = make(map[string]string)
	for i := 0; i < len(e.TagsJSON); i++ {
		pair := strings.TrimSpace(e.TagsJSON[i])
		if pair == "" {
			continue
		}

		arr := strings.SplitN(pair, "=", 2)
		if len(arr) != 2 {
			continue
		}

		e.TagsMap[arr[0]] = arr[1]
	}
}

func (e *AlertCurEvent) JsonTagsAndValue() map[string]string {
	v := reflect.ValueOf(e).Elem()
	t := v.Type()
	tags := make(map[string]string)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 获取 json tag
		tag := field.Tag.Get("json")
		if tag == "" {
			continue
		}

		// 处理类似 `json:",omitempty"` 或 `json:"-"` 的特殊情况
		tagParts := strings.Split(tag, ",")
		if tagParts[0] == "-" {
			continue
		}

		// 获取字段值并转换为字符串
		fieldValue := v.Field(i).Interface()
		var strValue string

		switch v := fieldValue.(type) {
		case string:
			strValue = v
		case int, int8, int16, int32, int64:
			strValue = fmt.Sprintf("%d", v)
		case float32, float64:
			strValue = fmt.Sprintf("%f", v)
		case bool:
			strValue = fmt.Sprintf("%v", v)
		case []string:
			b, _ := json.Marshal(v)
			strValue = string(b)
		case map[string]string:
			b, _ := json.Marshal(v)
			strValue = string(b)
		default:
			// 对于其他类型，尝试 JSON 序列化
			if b, err := json.Marshal(v); err == nil {
				strValue = string(b)
			} else {
				strValue = fmt.Sprintf("%v", v)
			}
		}

		// 如果没有指定 tag 名称，使用字段名作为 key
		if tagParts[0] == "" {
			tags[field.Name] = strValue
		} else {
			tags[tagParts[0]] = strValue
		}
	}
	return tags
}

type EventTriggerValues struct {
	ValuesWithUnit map[string]unit.FormattedValue `json:"values_with_unit"`
}

func (e *AlertCurEvent) TableName() string {
	return "alert_cur_event"
}

func (e *AlertCurEvent) Add(ctx *ctx.Context) error {
	return Insert(ctx, e)
}

type AggrRule struct {
	Type  string
	Value string
}

func (e *AlertCurEvent) ParseRule(field string) error {
	f := e.GetField(field)
	f = strings.TrimSpace(f)

	if f == "" {
		return nil
	}

	if field == "annotations" {
		err := json.Unmarshal([]byte(e.Annotations), &e.AnnotationsJSON)
		if err != nil {
			logger.Warningf("ruleid:%d failed to parse annotations: %v", e.RuleId, err)
			e.AnnotationsJSON = make(map[string]string)
			e.AnnotationsJSON["error"] = e.Annotations
		}

		for k, v := range e.AnnotationsJSON {
			f = v
			var defs = []string{
				"{{$labels := .TagsMap}}",
				"{{$value := .TriggerValue}}",
			}

			templateFuncMapCopy := tplx.NewTemplateFuncMap()
			templateFuncMapCopy["query"] = func(promql string, param ...int64) tplx.QueryResult {
				datasourceId := e.DatasourceId
				if len(param) > 0 {
					datasourceId = param[0]
				}
				value := tplx.Query(datasourceId, promql)
				return tplx.ConvertToQueryResult(value)
			}

			text := strings.Join(append(defs, f), "")
			t, err := template.New(fmt.Sprint(e.RuleId)).Funcs(templateFuncMapCopy).Parse(text)
			if err != nil {
				e.AnnotationsJSON[k] = fmt.Sprintf("failed to parse annotations: %v", err)
				continue
			}

			var body bytes.Buffer
			err = t.Execute(&body, e)
			if err != nil {
				e.AnnotationsJSON[k] = fmt.Sprintf("failed to parse annotations: %v", err)
				continue
			}

			e.AnnotationsJSON[k] = body.String()
		}

		b, err := json.Marshal(e.AnnotationsJSON)
		if err != nil {
			e.AnnotationsJSON = make(map[string]string)
			e.AnnotationsJSON["error"] = fmt.Sprintf("failed to parse annotations: %v", err)
		} else {
			e.Annotations = string(b)
		}

		return nil
	}

	var defs = []string{
		"{{$labels := .TagsMap}}",
		"{{$value := .TriggerValue}}",
		"{{$annotations := .AnnotationsJSON}}",
	}

	text := strings.Join(append(defs, f), "")
	t, err := template.New(fmt.Sprint(e.RuleId)).Funcs(template.FuncMap(tplx.TemplateFuncMap)).Parse(text)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	err = t.Execute(&body, e)
	if err != nil {
		return err
	}

	if field == "rule_name" {
		e.RuleName = body.String()
	}

	if field == "rule_note" {
		e.RuleNote = body.String()
	}

	return nil
}

func (e *AlertCurEvent) ParseURL(url string) (string, error) {

	f := strings.TrimSpace(url)

	if f == "" {
		return url, nil
	}

	var defs = []string{
		"{{$labels := .TagsMap}}",
		"{{$value := .TriggerValue}}",
		"{{$annotations := .AnnotationsJSON}}",
	}

	text := strings.Join(append(defs, f), "")
	t, err := template.New("callbackUrl" + fmt.Sprint(e.RuleId)).Funcs(template.FuncMap(tplx.TemplateFuncMap)).Parse(text)
	if err != nil {
		return url, nil
	}

	var body bytes.Buffer
	err = t.Execute(&body, e)
	if err != nil {
		return url, nil
	}

	return body.String(), nil
}

func parseAggrRules(rule string) []*AggrRule {
	aggrRules := strings.Split(rule, "::") // e.g. field:group_name::field:severity::tagkey:ident

	if len(aggrRules) == 0 {
		ginx.Bomb(http.StatusBadRequest, "rule empty")
	}

	rules := make([]*AggrRule, len(aggrRules))
	for i := 0; i < len(aggrRules); i++ {
		pair := strings.Split(aggrRules[i], ":")
		if len(pair) != 2 {
			ginx.Bomb(http.StatusBadRequest, "rule invalid")
		}

		if !(pair[0] == "field" || pair[0] == "tagkey") {
			ginx.Bomb(http.StatusBadRequest, "rule invalid")
		}

		rules[i] = &AggrRule{
			Type:  pair[0],
			Value: pair[1],
		}
	}
	return rules
}

func (e *AlertCurEvent) GenCardTitle(rule string) (string, error) {
	if strings.Contains(rule, "{{") {
		// 有 {{ 表示使用的是新的配置方式，使用 go template 进行格式化

		tmpl, err := template.New("card_title").Parse(rule)
		if err != nil {
			return fmt.Sprintf("failed to parse card title: %v", err), nil
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, e); err != nil {
			return fmt.Sprintf("failed to execute card title: %v", err), nil
		}
		return buf.String(), nil
	}

	rules := parseAggrRules(rule)
	arr := make([]string, len(rules))
	for i := 0; i < len(rules); i++ {
		rule := rules[i]

		if rule.Type == "field" {
			arr[i] = e.GetField(rule.Value)
		}

		if rule.Type == "tagkey" {
			arr[i] = e.GetTagValue(rule.Value)
		}

		if len(arr[i]) == 0 {
			arr[i] = "Others"
		}
	}
	return strings.Join(arr, "::"), nil
}

func (e *AlertCurEvent) GetTagValue(tagkey string) string {
	for _, tag := range e.TagsJSON {
		i := strings.Index(tag, tagkey+"=")
		if i >= 0 {
			return tag[len(tagkey+"="):]
		}
	}
	return ""
}

func (e *AlertCurEvent) GetField(field string) string {
	switch field {
	case "cluster":
		return e.Cluster
	case "group_id":
		return fmt.Sprint(e.GroupId)
	case "group_name":
		return e.GroupName
	case "rule_id":
		return fmt.Sprint(e.RuleId)
	case "rule_name":
		return e.RuleName
	case "rule_note":
		return e.RuleNote
	case "severity":
		return fmt.Sprint(e.Severity)
	case "runbook_url":
		return e.RunbookUrl
	case "target_ident":
		return e.TargetIdent
	case "target_note":
		return e.TargetNote
	case "callbacks":
		return e.Callbacks
	case "annotations":
		return e.Annotations
	default:
		return ""
	}
}

func (e *AlertCurEvent) ToHis(ctx *ctx.Context) *AlertHisEvent {
	isRecovered := 0
	var recoverTime int64 = 0
	if e.IsRecovered {
		isRecovered = 1
		recoverTime = e.LastEvalTime
	}

	return &AlertHisEvent{
		IsRecovered:      isRecovered,
		Cate:             e.Cate,
		Cluster:          e.Cluster,
		DatasourceId:     e.DatasourceId,
		GroupId:          e.GroupId,
		GroupName:        e.GroupName,
		Hash:             e.Hash,
		RuleId:           e.RuleId,
		RuleName:         e.RuleName,
		RuleProd:         e.RuleProd,
		RuleAlgo:         e.RuleAlgo,
		RuleNote:         e.RuleNote,
		Severity:         e.Severity,
		PromForDuration:  e.PromForDuration,
		PromQl:           e.PromQl,
		PromEvalInterval: e.PromEvalInterval,
		RuleConfig:       e.RuleConfig,
		RuleConfigJson:   e.RuleConfigJson,
		Callbacks:        e.Callbacks,
		RunbookUrl:       e.RunbookUrl,
		NotifyRecovered:  e.NotifyRecovered,
		NotifyChannels:   e.NotifyChannels,
		NotifyGroups:     e.NotifyGroups,
		Annotations:      e.Annotations,
		AnnotationsJSON:  e.AnnotationsJSON,
		TargetIdent:      e.TargetIdent,
		TargetNote:       e.TargetNote,
		TriggerTime:      e.TriggerTime,
		TriggerValue:     e.TriggerValue,
		Tags:             e.Tags,
		OriginalTags:     e.OriginalTags,
		RecoverTime:      recoverTime,
		LastEvalTime:     e.LastEvalTime,
		NotifyCurNumber:  e.NotifyCurNumber,
		FirstTriggerTime: e.FirstTriggerTime,
		NotifyRuleIds:    e.NotifyRuleIds,
	}
}

func (e *AlertCurEvent) DB2FE() error {
	e.NotifyChannelsJSON = strings.Fields(e.NotifyChannels)
	e.NotifyGroupsJSON = strings.Fields(e.NotifyGroups)
	e.CallbacksJSON = strings.Fields(e.Callbacks)
	e.TagsJSON = strings.Split(e.Tags, ",,")
	e.OriginalTagsJSON = strings.Split(e.OriginalTags, ",,")
	if err := json.Unmarshal([]byte(e.Annotations), &e.AnnotationsJSON); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(e.RuleConfig), &e.RuleConfigJson); err != nil {
		return err
	}

	e.TagsMap = make(map[string]string)
	for i := 0; i < len(e.TagsJSON); i++ {
		pair := strings.TrimSpace(e.TagsJSON[i])
		if pair == "" {
			continue
		}

		arr := strings.SplitN(pair, "=", 2)
		if len(arr) != 2 {
			continue
		}

		e.TagsMap[arr[0]] = arr[1]
	}

	return nil
}

func (e *AlertCurEvent) FE2DB() {
	e.NotifyChannels = strings.Join(e.NotifyChannelsJSON, " ")
	e.NotifyGroups = strings.Join(e.NotifyGroupsJSON, " ")
	e.Callbacks = strings.Join(e.CallbacksJSON, " ")
	e.Tags = strings.Join(e.TagsJSON, ",,")
	e.OriginalTags = strings.Join(e.OriginalTagsJSON, ",,")
	b, _ := json.Marshal(e.AnnotationsJSON)
	e.Annotations = string(b)

	b, _ = json.Marshal(e.RuleConfigJson)
	e.RuleConfig = string(b)

}

func (e *AlertCurEvent) DB2Mem() {
	e.IsRecovered = false
	e.NotifyGroupsJSON = strings.Fields(e.NotifyGroups)
	e.CallbacksJSON = strings.Fields(e.Callbacks)
	e.NotifyChannelsJSON = strings.Fields(e.NotifyChannels)
	e.TagsJSON = strings.Split(e.Tags, ",,")
	e.TagsMap = make(map[string]string)
	for i := 0; i < len(e.TagsJSON); i++ {
		pair := strings.TrimSpace(e.TagsJSON[i])
		if pair == "" {
			continue
		}

		arr := strings.SplitN(pair, "=", 2)
		if len(arr) != 2 {
			continue
		}

		e.TagsMap[arr[0]] = arr[1]
	}

	// 解决之前数据库中 FirstTriggerTime 为 0 的情况
	if e.FirstTriggerTime == 0 {
		e.FirstTriggerTime = e.TriggerTime
	}
}

func (e *AlertCurEvent) OverrideGlobalWebhook() bool {
	var rc RuleConfig
	if err := json.Unmarshal([]byte(e.RuleConfig), &rc); err != nil {
		logger.Warningf("failed to unmarshal rule config: %v", err)
		return false
	}
	return rc.OverrideGlobalWebhook
}

func FillRuleConfigTplName(ctx *ctx.Context, ruleConfig string) (interface{}, bool) {
	var config RuleConfig
	err := json.Unmarshal([]byte(ruleConfig), &config)
	if err != nil {
		logger.Warningf("failed to unmarshal rule config: %v", err)
		return nil, false
	}

	if len(config.TaskTpls) == 0 {
		return nil, false
	}

	for i := 0; i < len(config.TaskTpls); i++ {
		tpl, err := TaskTplGetById(ctx, config.TaskTpls[i].TplId)
		if err != nil {
			logger.Warningf("failed to get task tpl by id:%d, %v", config.TaskTpls[i].TplId, err)
			return nil, false
		}

		if tpl == nil {
			logger.Warningf("task tpl not found by id:%d", config.TaskTpls[i].TplId)
			return nil, false
		}
		config.TaskTpls[i].TplName = tpl.Title
	}
	return config, true
}

// for webui
func (e *AlertCurEvent) FillNotifyGroups(ctx *ctx.Context, cache map[int64]*UserGroup) error {
	// some user-group already deleted ?
	count := len(e.NotifyGroupsJSON)
	if count == 0 {
		e.NotifyGroupsObj = []*UserGroup{}
		return nil
	}

	for i := range e.NotifyGroupsJSON {
		id, err := strconv.ParseInt(e.NotifyGroupsJSON[i], 10, 64)
		if err != nil {
			continue
		}

		ug, has := cache[id]
		if has {
			e.NotifyGroupsObj = append(e.NotifyGroupsObj, ug)
			continue
		}

		ug, err = UserGroupGetById(ctx, id)
		if err != nil {
			return err
		}

		if ug != nil {
			e.NotifyGroupsObj = append(e.NotifyGroupsObj, ug)
			cache[id] = ug
		}
	}

	return nil
}

func AlertCurEventTotal(ctx *ctx.Context, prods []string, bgids []int64, stime, etime int64,
	severity []int64, dsIds []int64, cates []string, ruleId int64, query string, eventIds []int64) (int64, error) {
	session := DB(ctx).Model(&AlertCurEvent{})
	if stime != 0 && etime != 0 {
		session = session.Where("trigger_time between ? and ?", stime, etime)
	}
	if len(prods) != 0 {
		session = session.Where("rule_prod in ?", prods)
	}

	if len(bgids) > 0 {
		session = session.Where("group_id in ?", bgids)
	}

	if len(severity) > 0 {
		session = session.Where("severity in ?", severity)
	}

	if len(dsIds) > 0 {
		session = session.Where("datasource_id in ?", dsIds)
	}

	if len(cates) > 0 {
		session = session.Where("cate in ?", cates)
	}

	if ruleId > 0 {
		session = session.Where("rule_id = ?", ruleId)
	}

	if len(eventIds) > 0 {
		session = session.Where("id in ?", eventIds)
	}
	if query != "" {
		arr := strings.Fields(query)
		for i := 0; i < len(arr); i++ {
			qarg := "%" + arr[i] + "%"
			session = session.Where("rule_name like ? or tags like ?", qarg, qarg)
		}
	}

	return Count(session)
}

func AlertCurEventsGet(ctx *ctx.Context, prods []string, bgids []int64, stime, etime int64,
	severity []int64, dsIds []int64, cates []string, ruleId int64, query string, limit, offset int, eventIds []int64) (
	[]AlertCurEvent, error) {
	session := DB(ctx).Model(&AlertCurEvent{})

	if stime != 0 && etime != 0 {
		session = session.Where("trigger_time between ? and ?", stime, etime)
	}
	if len(prods) != 0 {
		session = session.Where("rule_prod in ?", prods)
	}

	if len(bgids) > 0 {
		session = session.Where("group_id in ?", bgids)
	}

	if len(severity) > 0 {
		session = session.Where("severity in ?", severity)
	}

	if len(dsIds) > 0 {
		session = session.Where("datasource_id in ?", dsIds)
	}

	if len(cates) > 0 {
		session = session.Where("cate in ?", cates)
	}

	if ruleId > 0 {
		session = session.Where("rule_id = ?", ruleId)
	}

	if len(eventIds) > 0 {
		session = session.Where("id in ?", eventIds)
	}

	if query != "" {
		arr := strings.Fields(query)
		for i := 0; i < len(arr); i++ {
			qarg := "%" + arr[i] + "%"
			session = session.Where("rule_name like ? or tags like ?", qarg, qarg)
		}
	}

	var lst []AlertCurEvent
	err := session.Order("trigger_time desc").Limit(limit).Offset(offset).Find(&lst).Error

	if err == nil {
		for i := 0; i < len(lst); i++ {
			lst[i].DB2FE()
		}
	}

	return lst, err
}

func AlertCurEventCountByRuleId(ctx *ctx.Context, rids []int64, stime, etime int64) map[int64]int64 {
	type Row struct {
		RuleId int64
		Cnt    int64
	}
	var rows []Row
	err := DB(ctx).Model(&AlertCurEvent{}).Select("rule_id, count(*) as cnt").
		Where("trigger_time between ? and ?", stime, etime).Group("rule_id").Find(&rows).Error
	if err != nil {
		logger.Errorf("Failed to count group by rule_id: %v", err)
		return nil
	}

	curEventTotalByRid := make(map[int64]int64, len(rids))
	for _, r := range rows {
		curEventTotalByRid[r.RuleId] = r.Cnt
	}
	return curEventTotalByRid
}

func AlertCurEventDel(ctx *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	return DB(ctx).Where("id in ?", ids).Delete(&AlertCurEvent{}).Error
}

func AlertCurEventDelByHash(ctx *ctx.Context, hash string) error {
	if !ctx.IsCenter {
		_, err := poster.GetByUrls[string](ctx, "/v1/n9e/alert-cur-events-del-by-hash?hash="+hash)
		return err
	}

	return DB(ctx).Where("hash = ?", hash).Delete(&AlertCurEvent{}).Error
}

func AlertCurEventExists(ctx *ctx.Context, where string, args ...interface{}) (bool, error) {
	return Exists(DB(ctx).Model(&AlertCurEvent{}).Where(where, args...))
}

func AlertCurEventGet(ctx *ctx.Context, where string, args ...interface{}) (*AlertCurEvent, error) {
	var lst []*AlertCurEvent
	err := DB(ctx).Where(where, args...).Find(&lst).Error
	if err != nil {
		return nil, err
	}

	if len(lst) == 0 {
		return nil, nil
	}

	lst[0].DB2FE()
	lst[0].FillNotifyGroups(ctx, make(map[int64]*UserGroup))

	return lst[0], nil
}

func AlertCurEventGetById(ctx *ctx.Context, id int64) (*AlertCurEvent, error) {
	return AlertCurEventGet(ctx, "id=?", id)
}

type AlertNumber struct {
	GroupId    int64
	GroupCount int64
}

// for busi_group list page
func AlertNumbers(ctx *ctx.Context, bgids []int64) (map[int64]int64, error) {
	ret := make(map[int64]int64)
	if len(bgids) == 0 {
		return ret, nil
	}

	var arr []AlertNumber
	err := DB(ctx).Model(&AlertCurEvent{}).Select("group_id", "count(*) as group_count").Where("group_id in ?", bgids).Group("group_id").Find(&arr).Error
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(arr); i++ {
		ret[arr[i].GroupId] = arr[i].GroupCount
	}

	return ret, nil
}

func AlertCurEventGetByIds(ctx *ctx.Context, ids []int64) ([]*AlertCurEvent, error) {
	var lst []*AlertCurEvent

	if len(ids) == 0 {
		return lst, nil
	}

	err := DB(ctx).Model(&AlertCurEvent{}).Where("id in ?", ids).Order("trigger_time desc").Find(&lst).Error
	if err == nil {
		for i := 0; i < len(lst); i++ {
			lst[i].DB2FE()
		}
	}

	return lst, err
}

func AlertCurEventGetByRuleIdAndDsId(ctx *ctx.Context, ruleId int64, datasourceId int64) ([]*AlertCurEvent, error) {
	if !ctx.IsCenter {
		lst, err := poster.GetByUrls[[]*AlertCurEvent](ctx, "/v1/n9e/alert-cur-events-get-by-rid?rid="+strconv.FormatInt(ruleId, 10)+"&dsid="+strconv.FormatInt(datasourceId, 10))
		if err == nil {
			for i := 0; i < len(lst); i++ {
				lst[i].FE2DB()
			}
		}
		return lst, err
	}

	var lst []*AlertCurEvent
	err := DB(ctx).Where("rule_id=? and datasource_id = ?", ruleId, datasourceId).Find(&lst).Error
	if err == nil {
		for i := 0; i < len(lst); i++ {
			lst[i].DB2FE()
		}
	}
	return lst, err
}

func AlertCurEventGetMap(ctx *ctx.Context, cluster string) (map[int64]map[string]struct{}, error) {
	session := DB(ctx).Model(&AlertCurEvent{})
	if cluster != "" {
		session = session.Where("datasource_id = ?", cluster)
	}

	var lst []*AlertCurEvent
	err := session.Select("rule_id", "hash").Find(&lst).Error
	if err != nil {
		return nil, err
	}

	ret := make(map[int64]map[string]struct{})
	for i := 0; i < len(lst); i++ {
		rid := lst[i].RuleId
		hash := lst[i].Hash
		if _, has := ret[rid]; has {
			ret[rid][hash] = struct{}{}
		} else {
			ret[rid] = make(map[string]struct{})
			ret[rid][hash] = struct{}{}
		}
	}

	return ret, nil
}

func (e *AlertCurEvent) UpdateFieldsMap(ctx *ctx.Context, fields map[string]interface{}) error {
	return DB(ctx).Model(e).Updates(fields).Error
}

func AlertCurEventUpgradeToV6(ctx *ctx.Context, dsm map[string]Datasource) error {
	var lst []*AlertCurEvent
	err := DB(ctx).Where("trigger_time > ?", time.Now().Unix()-3600*24*30).Find(&lst).Error
	if err != nil {
		return err
	}

	for i := 0; i < len(lst); i++ {
		ds, exists := dsm[lst[i].Cluster]
		if !exists {
			continue
		}
		lst[i].DatasourceId = ds.Id

		ruleConfig := PromRuleConfig{
			Queries: []PromQuery{
				{
					PromQl:   lst[i].PromQl,
					Severity: lst[i].Severity,
				},
			},
		}
		b, _ := json.Marshal(ruleConfig)
		lst[i].RuleConfig = string(b)

		if lst[i].RuleProd == "" {
			lst[i].RuleProd = METRIC
		}

		if lst[i].Cate == "" {
			lst[i].Cate = PROMETHEUS
		}

		err = lst[i].UpdateFieldsMap(ctx, map[string]interface{}{
			"datasource_id": lst[i].DatasourceId,
			"rule_config":   lst[i].RuleConfig,
			"rule_prod":     lst[i].RuleProd,
			"cate":          lst[i].Cate,
		})

		if err != nil {
			logger.Errorf("update alert rule:%d datasource ids failed, %v", lst[i].Id, err)
		}
	}
	return nil
}

// AlertCurEventGetsFromAlertMute find current events from db.
func AlertCurEventGetsFromAlertMute(ctx *ctx.Context, alertMute *AlertMute) ([]*AlertCurEvent, error) {
	var lst []*AlertCurEvent

	tx := DB(ctx).Where("group_id = ?", alertMute.GroupId)

	if len(alertMute.SeveritiesJson) != 0 {
		tx = tx.Where("severity IN (?)", alertMute.SeveritiesJson)
	}
	if len(alertMute.DatasourceIdsJson) != 0 && !IsAllDatasource(alertMute.DatasourceIdsJson) {
		tx = tx.Where("datasource_id IN (?)", alertMute.DatasourceIdsJson)
	}

	err := tx.Order("id desc").Find(&lst).Error
	return lst, err
}

func AlertCurEventStatistics(ctx *ctx.Context, stime time.Time) map[string]interface{} {
	stime24HoursAgoUnix := stime.Add(-24 * time.Hour).Unix()
	//Beginning of today
	stimeMidnightUnix := time.Date(stime.Year(), stime.Month(), stime.Day(), 0, 0, 0, 0, stime.Location()).Unix()
	///Monday of the current week, starting at 00:00
	daysToMonday := (int(stime.Weekday()) - 1 + 7) % 7 // (DayOfTheWeek - Monday(1) + DaysAWeek(7))/DaysAWeek(7)
	stimeOneWeekAgoUnix := time.Date(stime.Year(), stime.Month(), stime.Day()-daysToMonday, 0, 0, 0, 0, stime.Location()).Unix()

	var err error
	res := make(map[string]interface{})

	res["total"], err = Count(DB(ctx).Model(&AlertCurEvent{}))
	if err != nil {
		logger.Debugf("count alert current rule failed(total), %v", err)
	}

	res["total_24_ago"], err = Count(DB(ctx).Model(&AlertCurEvent{}).Where("trigger_time < ?", stime24HoursAgoUnix))
	if err != nil {
		logger.Debugf("count alert current rule failed(total_24ago), %v", err)
	}

	res["total_today"], err = Count(DB(ctx).Model(&AlertHisEvent{}).Where("trigger_time >= ? and is_recovered = ? ", stimeMidnightUnix, 0))
	if err != nil {
		logger.Debugf("count alert his rule failed(total_today), %v", err)
	}

	res["total_week"], err = Count(DB(ctx).Model(&AlertHisEvent{}).Where("trigger_time >= ? and is_recovered = ? ", stimeOneWeekAgoUnix, 0))
	if err != nil {
		logger.Debugf("count alert his rule failed(total_today), %v", err)
	}

	return res
}

func (e *AlertCurEvent) DeepCopy() *AlertCurEvent {
	eventCopy := *e

	// 复制指针字段
	if e.NotifyGroupsObj != nil {
		eventCopy.NotifyGroupsObj = make([]*UserGroup, len(e.NotifyGroupsObj))
		for i, group := range e.NotifyGroupsObj {
			if group != nil {
				groupCopy := *group
				eventCopy.NotifyGroupsObj[i] = &groupCopy
			}
		}
	}

	if e.NotifyUsersObj != nil {
		eventCopy.NotifyUsersObj = make([]*User, len(e.NotifyUsersObj))
		for i, user := range e.NotifyUsersObj {
			if user != nil {
				userCopy := *user
				eventCopy.NotifyUsersObj[i] = &userCopy
			}
		}
	}

	if e.Target != nil {
		targetCopy := *e.Target
		eventCopy.Target = &targetCopy
	}

	// 复制切片字段
	if e.CallbacksJSON != nil {
		eventCopy.CallbacksJSON = make([]string, len(e.CallbacksJSON))
		copy(eventCopy.CallbacksJSON, e.CallbacksJSON)
	}

	if e.NotifyChannelsJSON != nil {
		eventCopy.NotifyChannelsJSON = make([]string, len(e.NotifyChannelsJSON))
		copy(eventCopy.NotifyChannelsJSON, e.NotifyChannelsJSON)
	}

	if e.NotifyGroupsJSON != nil {
		eventCopy.NotifyGroupsJSON = make([]string, len(e.NotifyGroupsJSON))
		copy(eventCopy.NotifyGroupsJSON, e.NotifyGroupsJSON)
	}

	if e.TagsJSON != nil {
		eventCopy.TagsJSON = make([]string, len(e.TagsJSON))
		copy(eventCopy.TagsJSON, e.TagsJSON)
	}

	if e.TagsMap != nil {
		eventCopy.TagsMap = make(map[string]string, len(e.TagsMap))
		for k, v := range e.TagsMap {
			eventCopy.TagsMap[k] = v
		}
	}

	if e.OriginalTagsJSON != nil {
		eventCopy.OriginalTagsJSON = make([]string, len(e.OriginalTagsJSON))
		copy(eventCopy.OriginalTagsJSON, e.OriginalTagsJSON)
	}

	if e.AnnotationsJSON != nil {
		eventCopy.AnnotationsJSON = make(map[string]string, len(e.AnnotationsJSON))
		for k, v := range e.AnnotationsJSON {
			eventCopy.AnnotationsJSON[k] = v
		}
	}

	if e.ExtraInfo != nil {
		eventCopy.ExtraInfo = make([]string, len(e.ExtraInfo))
		copy(eventCopy.ExtraInfo, e.ExtraInfo)
	}

	if e.ExtraInfoMap != nil {
		eventCopy.ExtraInfoMap = make([]map[string]string, len(e.ExtraInfoMap))
		for i, m := range e.ExtraInfoMap {
			if m != nil {
				eventCopy.ExtraInfoMap[i] = make(map[string]string, len(m))
				for k, v := range m {
					eventCopy.ExtraInfoMap[i][k] = v
				}
			}
		}
	}

	if e.NotifyRuleIds != nil {
		eventCopy.NotifyRuleIds = make([]int64, len(e.NotifyRuleIds))
		copy(eventCopy.NotifyRuleIds, e.NotifyRuleIds)
	}

	eventCopy.RuleConfigJson = e.RuleConfigJson
	eventCopy.ExtraConfig = e.ExtraConfig

	return &eventCopy
}
