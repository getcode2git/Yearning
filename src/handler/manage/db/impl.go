package db

import (
	"Yearning-go/src/handler/commom"
	"Yearning-go/src/lib"
	"Yearning-go/src/model"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
)

const (
	CONN_TEST_SUCCESS = "数据库实例连接成功！"
	DB_SAVE_SUCCESS   = "连接名添加成功！"
	ERR_DB_SAVE       = "config.toml文件中SecretKey值必须为16位！"
	DB_EDIT_SUCCESS   = "数据源信息已更新!"
)

type CommonDBPost struct {
	Encrypt       bool                 `json:"encrypt"`
	Tp            string               `json:"tp"`
	DB            model.CoreDataSource `json:"db"`
	ExcludeDbList []string             `json:"exclude_db_list"`
	WordList      []string             `json:"word_list"`
}

func ConnTest(u *model.CoreDataSource) error {
	db, err := gorm.Open("mysql", fmt.Sprintf("%s:%s@(%s:%d)/?charset=utf8&parseTime=True&loc=Local", u.Username, u.Password, u.IP, u.Port))
	defer func() {
		_ = db.Close()
	}()
	if err != nil {
		return err
	}
	return nil
}

func SuperEditSource(source *model.CoreDataSource) commom.Resp {

	if source.Password != "" && lib.Decrypt(source.Password) == "" {
		model.DB().Model(&model.CoreDataSource{}).Where("source_id =?", source.SourceId).Update(&model.CoreDataSource{Password: lib.Encrypt(source.Password)})
	}

	model.DB().Model(&model.CoreDataSource{}).Where("source_id =?", source.SourceId).Update(map[string]interface{}{
		"id_c":               source.IDC,
		"ip":                 source.IP,
		"port":               source.Port,
		"source":             source.Source,
		"username":           source.Username,
		"is_query":           source.IsQuery,
		"flow_id":            source.FlowID,
		"exclude_db_list":    source.ExcludeDbList,
		"principal":          source.Principal,
		"insulate_word_list": source.InsulateWordList,
	})
	var k []model.CoreRoleGroup
	model.DB().Find(&k)
	for i := range k {
		var p model.PermissionList
		if err := json.Unmarshal(k[i].Permissions, &p); err != nil {
			return commom.ERR_COMMON_MESSAGE(err)
		}
		if source.IsQuery == 0 {
			p.QuerySource = lib.ResearchDel(p.QuerySource, source.Source)
		}
		if source.IsQuery == 1 {
			p.DDLSource = lib.ResearchDel(p.DDLSource, source.Source)
			p.DMLSource = lib.ResearchDel(p.DMLSource, source.Source)
		}
		r, _ := json.Marshal(p)
		model.DB().Model(&model.CoreRoleGroup{}).Where("id =?", k[i].ID).Update(model.CoreRoleGroup{Permissions: r})
	}
	return commom.SuccessPayLoadToMessage(DB_EDIT_SUCCESS)
}

func SuperCreateSource(source *model.CoreDataSource) commom.Resp {
	source.Password = lib.Encrypt(source.Password)
	if source.Password != "" {
		source.SourceId = uuid.New().String()
		model.DB().Create(source)
		return commom.SuccessPayLoadToMessage(DB_SAVE_SUCCESS)
	}
	return commom.SuccessPayLoadToMessage(ERR_DB_SAVE)
}

func SuperTestDBConnect(source *model.CoreDataSource) commom.Resp {
	if err := ConnTest(source); err != nil {
		return commom.ERR_COMMON_MESSAGE(err)
	}
	return commom.SuccessPayLoadToMessage(CONN_TEST_SUCCESS)
}
