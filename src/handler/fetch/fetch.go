// Copyright 2019 HenryYee.
//
// Licensed under the AGPL, Version 3.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.gnu.org/licenses/agpl-3.0.en.html
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package fetch

import (
	"Yearning-go/src/engine"
	"Yearning-go/src/handler/commom"
	tpl2 "Yearning-go/src/handler/manage/tpl"
	"Yearning-go/src/lib"
	"Yearning-go/src/model"
	"encoding/json"
	"errors"
	"github.com/cookieY/yee"
	"golang.org/x/net/websocket"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

func FetchIDC(c yee.Context) (err error) {
	return c.JSON(http.StatusOK, commom.SuccessPayload(model.GloOther.IDC))

}

func FetchIsQueryAudit(c yee.Context) (err error) {
	return c.JSON(http.StatusOK, commom.SuccessPayload(map[string]interface{}{
		"status": model.GloOther.Query,
		"export": model.GloOther.Export,
	}))
}

func FetchQueryStatus(c yee.Context) (err error) {
	var check model.CoreQueryOrder
	t := new(lib.Token).JwtParse(c)
	model.DB().Model(model.CoreQueryOrder{}).Where("username =?", t.Username).Last(&check)
	if check.Status == 2 {
		isExpire := lib.TimeDifference(check.ApprovalTime)
		if isExpire {
			model.DB().Model(model.CoreQueryOrder{}).Where("work_id =?", check.WorkId).Update(&model.CoreSqlOrder{Status: 3})
		}
		return c.JSON(http.StatusOK, commom.SuccessPayload(isExpire))
	}

	return c.JSON(http.StatusOK, commom.SuccessPayload(true))
}

func FetchSource(c yee.Context) (err error) {

	u := new(_FetchBind)
	if err := c.Bind(u); err != nil {
		return err
	}
	if reflect.DeepEqual(u, _FetchBind{}) {
		return
	}

	var s model.CoreGrained
	var groups []string
	var source []model.CoreDataSource

	user := new(lib.Token).JwtParse(c)

	model.DB().Where("username =?", user.Username).First(&s)
	if err := json.Unmarshal(s.Group, &groups); err != nil {
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, commom.ERR_REQ_BIND)
	}
	p := lib.MultiUserRuleMarge(groups)
	switch u.Tp {
	case "count":
		return c.JSON(http.StatusOK, commom.SuccessPayload(map[string]interface{}{"ddl": len(p.DDLSource), "dml": len(p.DMLSource), "query": len(p.QuerySource)}))
	case "dml":
		model.DB().Select("source,id_c,source_id").Where("source_id IN (?)", p.DMLSource).Find(&source)
	case "ddl":
		model.DB().Select("source,id_c,source_id").Where("source_id IN (?)", p.DDLSource).Find(&source)
	case "query":
		var ord model.CoreQueryOrder
		// 如果打开查询审核,判断该用户是否存在查询中的工单.如果存在则直接返回该查询工单允许的数据源
		if model.GloOther.Query && !model.DB().Model(model.CoreQueryOrder{}).Where("username =? and `status` =2", user.Username).Last(&ord).RecordNotFound() {
			model.DB().Select("source,id_c,source_id").Where("source_id =?", ord.SourceId).Find(&source)
		} else {
			model.DB().Select("source,id_c,source_id").Where("source_id IN (?)", p.QuerySource).Find(&source)
		}
	case "all":
		model.DB().Select("source,id_c,source_id").Find(&source)
	case "idc":
		model.DB().Select("source,source_id").Where("id_c = ?", u.IDC).Find(&source)
	}
	return c.JSON(http.StatusOK, commom.SuccessPayload(source))
}

func FetchAuditSteps(c yee.Context) (err error) {
	u := c.QueryParam("source_id")
	unescape, _ := url.QueryUnescape(u)
	var s model.CoreDataSource
	var tpl model.CoreWorkflowTpl
	var whoIsAuditor []tpl2.Tpl
	model.DB().Model(model.CoreDataSource{}).Where("source_id = ?", unescape).First(&s)
	if model.DB().Model(model.CoreWorkflowTpl{}).Where("id =?", s.FlowID).Find(&tpl).RecordNotFound() {
		return c.JSON(http.StatusOK, commom.ERR_COMMON_MESSAGE(errors.New("数据源没有添加流程!无法提交工单")))
	}
	_ = json.Unmarshal(tpl.Steps, &whoIsAuditor)

	return c.JSON(http.StatusOK, commom.SuccessPayload(whoIsAuditor))

}

func FetchHighLight(c yee.Context) (err error) {
	var s model.CoreDataSource
	model.DB().Where("source_id =?", c.QueryParam("source_id")).First(&s)
	return c.JSON(http.StatusOK, commom.SuccessPayload(commom.Highlight(&s)))
}

func FetchBase(c yee.Context) (err error) {

	u := new(_FetchBind)
	if err := c.Bind(u); err != nil {
		return err
	}
	if reflect.DeepEqual(u, _FetchBind{}) {
		return
	}
	var s model.CoreDataSource

	unescape, _ := url.QueryUnescape(u.SourceId)

	model.DB().Where("source_id =?", unescape).First(&s)

	result, err := commom.ScanDataRows(s, "", "SHOW DATABASES;", "Schema", false, false)

	if err != nil {
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, commom.ERR_COMMON_MESSAGE(err))
	}
	if u.Hide {
		var _t []string
		mp := lib.MapOn(strings.Split(s.ExcludeDbList, ","))
		for _, i := range result.Results {
			if _, ok := mp[i]; !ok {
				_t = append(_t, i)
			}
		}
		result.Results = _t
	}
	return c.JSON(http.StatusOK, commom.SuccessPayload(map[string]interface{}{"results": result.Results}))
}

func FetchTable(c yee.Context) (err error) {
	u := new(_FetchBind)
	if err = c.Bind(u); err != nil {
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, commom.ERR_REQ_BIND)
	}
	var s model.CoreDataSource
	unescape, _ := url.QueryUnescape(u.SourceId)
	model.DB().Where("source_id =?", unescape).First(&s)

	result, err := commom.ScanDataRows(s, u.DataBase, "SHOW TABLES;", "Table", false, false)

	if err != nil {
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, commom.ERR_COMMON_MESSAGE(err))
	}
	return c.JSON(http.StatusOK, commom.SuccessPayload(map[string]interface{}{"results": result.Results}))
}

func FetchTableInfo(c yee.Context) (err error) {
	u := new(_FetchBind)
	if err = c.Bind(u); err != nil {
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, commom.ERR_REQ_BIND)
	}

	if u.DataBase != "" && u.Table != "" {
		if err := u.FetchTableFieldsOrIndexes(); err != nil {
			c.Logger().Critical(err.Error())
		}
		return c.JSON(http.StatusOK, commom.SuccessPayload(map[string]interface{}{"rows": u.Rows, "idx": u.Idx}))
	}
	return c.JSON(http.StatusOK, commom.ERR_COMMON_MESSAGE(errors.New("请选择库名以及表名后再点击获取表结构信息")))
}

func FetchSQLTest(c yee.Context) (err error) {
	u := new(commom.SQLTest)
	if err = c.Bind(u); err != nil {
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, commom.ERR_REQ_BIND)
	}
	var s model.CoreDataSource
	model.DB().Where("source_id =?", u.SourceId).First(&s)
	var rs []engine.Record
	if client := lib.NewRpc(); client != nil {
		if err := client.Call("Engine.Check", engine.CheckArgs{
			SQL:      u.SQL,
			Schema:   u.Database,
			IP:       s.IP,
			Username: s.Username,
			Port:     s.Port,
			Password: lib.Decrypt(s.Password),
			Kind:     u.Kind,
			Lang:     "zh-cn",
			Rule:     model.GloRole,
		}, &rs); err != nil {
			return c.JSON(http.StatusOK, commom.ERR_RPC)
		}
		return c.JSON(http.StatusOK, commom.SuccessPayload(rs))
	}
	return c.JSON(http.StatusOK, commom.ERR_RPC)
}

func FetchOrderDetailList(c yee.Context) (err error) {
	expr := new(PageSizeRef)
	if err := c.Bind(expr); err != nil {
		return c.JSON(http.StatusOK, commom.ERR_COMMON_MESSAGE(err))
	}
	var record []model.CoreSqlRecord
	var count int
	start, end := lib.Paging(expr.Page, expr.PageSize)
	model.DB().Model(&model.CoreSqlRecord{}).Where("work_id =?", expr.WorkId).Count(&count).Offset(start).Limit(end).Find(&record)
	return c.JSON(http.StatusOK, commom.SuccessPayload(map[string]interface{}{"record": record, "count": count}))
}

func FetchOrderDetailRollSQL(c yee.Context) (err error) {
	workId := c.QueryParam("work_id")
	var roll []model.CoreRollback
	var count int
	model.DB().Select("`sql`").Model(model.CoreRollback{}).Where("work_id =?", workId).Count(&count).Find(&roll)
	return c.JSON(http.StatusOK, commom.SuccessPayload(map[string]interface{}{"sql": roll, "count": count}))
}

func FetchUndo(c yee.Context) (err error) {
	u := c.QueryParam("work_id")
	user := new(lib.Token).JwtParse(c)
	var undo model.CoreSqlOrder
	if model.DB().Where(UNDO_EXPR, user.Username, u, 2).First(&undo).RecordNotFound() {
		return c.JSON(http.StatusOK, UNDO_MESSAGE_ERROR)
	}
	lib.MessagePush(undo.WorkId, 6, "")
	model.DB().Where(UNDO_EXPR, user.Username, u, 2).Delete(&model.CoreSqlOrder{})
	return c.JSON(http.StatusOK, commom.SuccessPayLoadToMessage(UNDO_MESSAGE_SUCCESS))
}

func FetchMergeDDL(c yee.Context) (err error) {
	req := new(referOrder)
	if err = c.Bind(req); err != nil {
		return c.JSON(http.StatusOK, commom.ERR_COMMON_MESSAGE(err))
	}
	var m string
	if req.SQLs != "" {
		if client := lib.NewRpc(); client != nil {
			if err := client.Call("Engine.MergeAlterTables", req.SQLs, &m); err != nil {
				return c.JSON(http.StatusOK, commom.ERR_SOAR_ALTER_MERGE(err))
			}
			return c.JSON(http.StatusOK, commom.SuccessPayload(m))
		}
	}
	return c.JSON(http.StatusOK, commom.ERR_SOAR_ALTER_MERGE(err))
}

func FetchSQLInfo(c yee.Context) (err error) {
	workId := c.QueryParam("work_id")
	var sql model.CoreSqlOrder
	model.DB().Select("`sql`").Where("work_id =?", workId).First(&sql)
	return c.JSON(http.StatusOK, commom.SuccessPayload(map[string]interface{}{"sqls": sql.SQL}))
}

func FetchStepsProfile(c yee.Context) (err error) {
	workId := c.QueryParam("work_id")
	var s []model.CoreWorkflowDetail
	model.DB().Where("work_id = ?", workId).Find(&s)
	return c.JSON(http.StatusOK, commom.SuccessPayload(s))
}

func FetchBoard(c yee.Context) (err error) {
	var board model.CoreGlobalConfiguration
	model.DB().Select("board").First(&board)
	return c.JSON(http.StatusOK, commom.SuccessPayload(board))
}

func FetchOrderComment(c yee.Context) (err error) {
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		workId := c.QueryParam("work_id")
		var msg string
		valid, err := lib.WSTokenIsValid(ws.Request().Header.Get("Sec-WebSocket-Protocol"))
		if err != nil {
			c.Logger().Error(err)
			return
		}
		if valid {
			for {
				if workId != "" {
					var comment []model.CoreOrderComment
					model.DB().Model(model.CoreOrderComment{}).Where("work_id =?", workId).Find(&comment)
					err := websocket.Message.Send(ws, lib.ToJson(comment))
					if err != nil {
						c.Logger().Error(err)
						break
					}
				}
				if err := websocket.Message.Receive(ws, &msg); err != nil {
					break
				}
				if msg == commom.CLOSE {
					break
				}
			}
		}

	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func PostOrderComment(c yee.Context) (err error) {
	u := new(model.CoreOrderComment)
	if err := c.Bind(u); err != nil {
		return c.JSON(http.StatusOK, commom.ERR_REQ_BIND)
	}
	t := new(lib.Token).JwtParse(c)
	u.Time = time.Now().Format("2006-01-02 15:04")
	u.Username = t.Username
	model.DB().Model(model.CoreOrderComment{}).Create(u)
	return c.JSON(http.StatusOK, commom.SuccessPayLoadToMessage(COMMENT_IS_POST))
}

func FetchUserGroups(c yee.Context) (err error) {
	user := new(lib.Token).JwtParse(c)
	toUser := c.QueryParam("user")
	if user.Username != "admin" && user.Username != toUser {
		return c.JSON(http.StatusOK, commom.ERR_COMMON_MESSAGE(errors.New("非法获取信息")))
	}
	var (
		p      model.CoreGrained
		g      []model.CoreRoleGroup
		groups []string
		//s         model.CoreGlobalConfiguration
	)

	//model.DB().Select("username,rule,department,real_name,email").Where("username =?", user).Find(&u)
	model.DB().Select("`group`").Where("username =?", toUser).First(&p)
	model.DB().Select("`group_id`,`name`").Find(&g)
	err = json.Unmarshal(p.Group, &groups)
	if err != nil {
		return c.JSON(http.StatusOK, commom.ERR_COMMON_MESSAGE(err))
	}
	//model.DB().Select("stmt").First(&s)
	return c.JSON(http.StatusOK, commom.SuccessPayload(map[string]interface{}{"own": p.Group, "groups": g}))
}

func FetchOrderState(c yee.Context) (err error) {
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		workId := c.QueryParam("work_id")
		var msg string
		valid, err := lib.WSTokenIsValid(ws.Request().Header.Get("Sec-WebSocket-Protocol"))
		if err != nil {
			c.Logger().Error(err)
			return
		}
		if valid {
			for {
				if workId != "" {
					var order model.CoreSqlOrder
					model.DB().Model(model.CoreSqlOrder{}).Select("status").Where("work_id =?", workId).First(&order)
					err := websocket.Message.Send(ws, lib.ToJson(order.Status))
					if err != nil {
						c.Logger().Error(err)
						break
					}
				}
				if err := websocket.Message.Receive(ws, &msg); err != nil {
					break
				}
				if msg == commom.CLOSE {
					break
				}
			}
		}
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func FetchUserInfo(c yee.Context) (err error) {
	t := new(lib.Token).JwtParse(c)
	var userInfo model.CoreAccount
	model.DB().Select("department,username,real_name,email").Model(model.CoreAccount{}).Where("username =?", t.Username).First(&userInfo)
	return c.JSON(http.StatusOK, commom.SuccessPayload(userInfo))
}
