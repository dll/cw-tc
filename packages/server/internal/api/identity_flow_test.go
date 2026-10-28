package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestStudentRegistrationLoginDataChain(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	if _, err := staff.Create(context.Background(), database, "ops", "运营管理员", staff.RoleAdminOps, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()

	invalid := performJSON(t, r, http.MethodPost, "/api/v1/auth/register", `{"name":"样例学员","phone":"12812345678","id_card":"110101199001011237","password":"password-1"}`)
	if !strings.Contains(invalid.Body.String(), `"code":10002`) {
		t.Fatalf("非法手机号未拒绝: %s", invalid.Body.String())
	}
	invalid = performJSON(t, r, http.MethodPost, "/api/v1/auth/register", `{"name":"样例学员","phone":"13812345678","id_card":"110101199001011235","password":"password-1"}`)
	if !strings.Contains(invalid.Body.String(), `"code":10002`) {
		t.Fatalf("非法证件号未拒绝: %s", invalid.Body.String())
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM student`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("非法注册写入数据库: %d %v", count, err)
	}

	register := performJSON(t, r, http.MethodPost, "/api/v1/auth/register", `{"name":"样例学员","phone":"13812345678","id_card":"110101199001011237","password":"password-1"}`)
	if register.Code != http.StatusOK || !strings.Contains(register.Body.String(), `"code":0`) {
		t.Fatalf("合法注册失败: %d %s", register.Code, register.Body.String())
	}
	if strings.Contains(register.Body.String(), `"token"`) {
		t.Fatalf("待审核注册不得建立会话: %s", register.Body.String())
	}
	var verified sql.NullString
	if err := database.QueryRow(`SELECT phone_verified_at FROM student WHERE phone=$1`, "13812345678").Scan(&verified); err != nil || verified.Valid {
		t.Fatalf("未经过短信验证却标记已验证: %v %v", verified, err)
	}
	login := performJSON(t, r, http.MethodPost, "/api/v1/auth/login", `{"phone":"13812345678","password":"password-1"}`)
	if login.Code != http.StatusForbidden || !strings.Contains(login.Body.String(), "待运营管理员审核") {
		t.Fatalf("未审核账号应禁止登录: %d %s", login.Code, login.Body.String())
	}
	adminLogin := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"ops","password":"Temp@123"}`)
	adminToken := extractJSONField(t, adminLogin.Body.String(), "token")
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/not-a-number/approve-registration", `{"reason":"资料审核通过","confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("非法学员编号应拒绝: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/approve-registration", `{"reason":" ","confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("空审核事由应拒绝: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/approve-registration", `{"reason":"审核","confirm":false}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("缺少二次确认应拒绝: %d %s", got.Code, got.Body.String())
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users?type=student", "", adminToken)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"registration_status":"pending"`) || !strings.Contains(list.Body.String(), `"total":1`) {
		t.Fatalf("新学员未出现在管理员列表: %d %s", list.Code, list.Body.String())
	}
	pending := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/registration-pending", "", adminToken)
	if pending.Code != http.StatusOK || !strings.Contains(pending.Body.String(), `"count":1`) {
		t.Fatalf("注册待办未产生: %d %s", pending.Code, pending.Body.String())
	}
	var studentID int64
	if err := database.QueryRow(`SELECT id FROM student WHERE phone=$1`, "13812345678").Scan(&studentID); err != nil {
		t.Fatal(err)
	}
	approved := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/"+strconv.FormatInt(studentID, 10)+"/approve-registration", `{"reason":"资料审核通过","confirm":true}`, adminToken)
	if approved.Code != http.StatusOK {
		t.Fatalf("管理员审核失败: %d %s", approved.Code, approved.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/"+strconv.FormatInt(studentID, 10)+"/approve-registration", `{"reason":"重复审核","confirm":true}`, adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("重复审核应返回 404: %d %s", got.Code, got.Body.String())
	}
	pending = performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/registration-pending", "", adminToken)
	if pending.Code != http.StatusOK || !strings.Contains(pending.Body.String(), `"count":0`) {
		t.Fatalf("审核后待办未清除: %d %s", pending.Code, pending.Body.String())
	}
	login = performJSON(t, r, http.MethodPost, "/api/v1/auth/login", `{"phone":"13812345678","password":"password-1"}`)
	if login.Code != http.StatusOK || !strings.Contains(login.Body.String(), `"token"`) {
		t.Fatalf("审核通过后应允许登录: %d %s", login.Code, login.Body.String())
	}
}
