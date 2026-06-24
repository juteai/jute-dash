package repository

type HouseholdSettingsDB struct {
	ID                      string `gorm:"primaryKey;column:id"`
	Name                    string `gorm:"column:name"`
	DisplayTheme            string `gorm:"column:display_theme"`
	DisplayAccentColor      string `gorm:"column:display_accent_color"`
	DisplayIdleMode         string `gorm:"column:display_idle_mode"`
	SetupCompleted          int    `gorm:"column:setup_completed;default:0"`
	CreatedAt               string `gorm:"column:created_at"`
	UpdatedAt               string `gorm:"column:updated_at"`
	DisplayColorMode        string `gorm:"column:display_color_mode;default:'system'"`
	DisplayThemeID          string `gorm:"column:display_theme_id;default:'jute-mono'"`
	DisplayDensity          string `gorm:"column:display_density;default:'comfortable'"`
	DisplayMotion           string `gorm:"column:display_motion;default:'full'"`
	DisplayBackgroundJSON   string `gorm:"column:display_background_json;default:'{}'"`
	DisplayWidgetChromeJSON string `gorm:"column:display_widget_chrome_json;default:'{}'"`
}

func (HouseholdSettingsDB) TableName() string {
	return "household_settings"
}

type DeviceProfileDB struct {
	ID              string `gorm:"primaryKey;column:id"`
	Name            string `gorm:"column:name"`
	InteractionMode string `gorm:"column:interaction_mode"`
	LayoutProfileID string `gorm:"column:layout_profile_id"`
	SettingsJSON    string `gorm:"column:settings_json"`
	CreatedAt       string `gorm:"column:created_at"`
	UpdatedAt       string `gorm:"column:updated_at"`
}

func (DeviceProfileDB) TableName() string {
	return "device_profiles"
}

type LayoutProfileDB struct {
	ID              string `gorm:"primaryKey;column:id"`
	DeviceProfileID string `gorm:"column:device_profile_id"`
	Name            string `gorm:"column:name"`
	SettingsJSON    string `gorm:"column:settings_json"`
	CreatedAt       string `gorm:"column:created_at"`
	UpdatedAt       string `gorm:"column:updated_at"`
}

func (LayoutProfileDB) TableName() string {
	return "layout_profiles"
}

type RoomDB struct {
	ID        string `gorm:"primaryKey;column:id"`
	Name      string `gorm:"column:name"`
	Summary   string `gorm:"column:summary"`
	Status    string `gorm:"column:status"`
	SortOrder int    `gorm:"column:sort_order"`
	CreatedAt string `gorm:"column:created_at"`
	UpdatedAt string `gorm:"column:updated_at"`
}

func (RoomDB) TableName() string {
	return "rooms"
}

type TileDB struct {
	ID        string `gorm:"primaryKey;column:id"`
	Kind      string `gorm:"column:kind"`
	Label     string `gorm:"column:label"`
	Value     string `gorm:"column:value"`
	Detail    string `gorm:"column:detail"`
	SortOrder int    `gorm:"column:sort_order"`
	CreatedAt string `gorm:"column:created_at"`
	UpdatedAt string `gorm:"column:updated_at"`
}

func (TileDB) TableName() string {
	return "tiles"
}

type AdapterConnectionDB struct {
	ID            string `gorm:"primaryKey;column:id"`
	Kind          string `gorm:"column:kind"`
	Name          string `gorm:"column:name"`
	SettingsJSON  string `gorm:"column:settings_json"`
	SecretRefJSON string `gorm:"column:secret_ref_json"`
	Enabled       int    `gorm:"column:enabled"`
	CreatedAt     string `gorm:"column:created_at"`
	UpdatedAt     string `gorm:"column:updated_at"`
}

func (AdapterConnectionDB) TableName() string {
	return "adapter_connections"
}

type SettingAuditLogDB struct {
	ID           uint   `gorm:"primaryKey;autoIncrement;column:id"`
	Actor        string `gorm:"column:actor"`
	Action       string `gorm:"column:action"`
	Target       string `gorm:"column:target"`
	MetadataJSON string `gorm:"column:metadata_json"`
	CreatedAt    string `gorm:"column:created_at"`
}

func (SettingAuditLogDB) TableName() string {
	return "setting_audit_log"
}
