package commerce

import (
	"context"
	"testing"
	"time"
)

func TestCreateOrderSnapshotsCoursePrice(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	if _, err := database.Exec(`UPDATE course SET price_cents=12900, name='样例课程（定价）' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	service := NewService(database)
	order, err := service.CreateOrderFromCourse(context.Background(), 1, 1, "price-1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if order.CourseName != "样例课程（定价）" || order.PriceCents != 12900 || order.AmountCents != 12900 {
		t.Fatalf("价格快照错误: %+v", order)
	}
}
