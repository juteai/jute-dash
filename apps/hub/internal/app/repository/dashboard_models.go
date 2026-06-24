package repository

type WidgetPackDB struct {
	ID           string `gorm:"primaryKey;column:id"`
	Name         string `gorm:"column:name"`
	Version      string `gorm:"column:version"`
	ManifestJSON string `gorm:"column:manifest_json"`
	InstalledAt  string `gorm:"column:installed_at"`
	UpdatedAt    string `gorm:"column:updated_at"`
}

func (WidgetPackDB) TableName() string {
	return "widget_packs"
}

type WidgetInstanceDB struct {
	ID                 string `gorm:"primaryKey;column:id"`
	ScreenID           string `gorm:"column:screen_id;default:'home'"`
	Kind               string `gorm:"column:kind"`
	Title              string `gorm:"column:title"`
	LayoutProfileID    string `gorm:"column:layout_profile_id"`
	X                  int    `gorm:"column:x"`
	Y                  int    `gorm:"column:y"`
	W                  int    `gorm:"column:w"`
	H                  int    `gorm:"column:h"`
	MinW               int    `gorm:"column:min_w"`
	MinH               int    `gorm:"column:min_h"`
	Size               string `gorm:"column:size"`
	Mode               string `gorm:"column:mode;default:'ui'"`
	SettingsJSON       string `gorm:"column:settings_json"`
	ConnectionRefsJSON string `gorm:"column:connection_refs_json"`
	Visible            int    `gorm:"column:visible"`
	SortOrder          int    `gorm:"column:sort_order"`
	CreatedAt          string `gorm:"column:created_at"`
	UpdatedAt          string `gorm:"column:updated_at"`
}

func (WidgetInstanceDB) TableName() string {
	return "widget_instances"
}

type WidgetPermissionDB struct {
	WidgetInstanceID string `gorm:"primaryKey;column:widget_instance_id"`
	Permission       string `gorm:"primaryKey;column:permission"`
	Granted          int    `gorm:"column:granted"`
	UpdatedAt        string `gorm:"column:updated_at"`
}

func (WidgetPermissionDB) TableName() string {
	return "widget_permissions"
}
