package manifest

import "github.com/lixenwraith/vi-fighter/internal/engine"

// ComponentDef defines a component for registration and store generation
type ComponentDef struct {
	Field string // ComponentStore field name (e.g., "Drain")
	Type  string // Type name without package (e.g., "DrainComponent")
}

// SystemDef defines a system for registration
// Order in slice determines ActiveSystems() order
type SystemDef struct {
	Name        string // Registry key (e.g., "drain")
	Constructor string // Constructor name without package (e.g., "NewDrainSystem")
	Domain      engine.SystemDomain
}

// RendererDef defines a renderer for registration
// Order in slice determines ActiveRenderers() order
type RendererDef struct {
	Name        string // Registry key (e.g., "drain")
	Constructor string // Constructor name without package (e.g., "NewDrainRenderer")
	Priority    string // Priority constant without package (e.g., "PriorityDrain")
}

// Components is the authoritative component list
// Generator produces: ComponentStore, GetComponentStore(), RegisterComponents()
var Components = []ComponentDef{
	// --- Core Gameplay ---
	{"Glyph", "GlyphComponent"},
	{"Sigil", "SigilComponent"},
	{"Nugget", "NuggetComponent"},
	{"Cursor", "CursorComponent"},
	{"Protection", "ProtectionComponent"},
	{"Kinetic", "KineticComponent"},
	{"Wall", "WallComponent"},
	{"Loot", "LootComponent"},
	{"Gateway", "GatewayComponent"},

	// --- Player State ---
	{"Energy", "EnergyComponent"},
	{"Heat", "HeatComponent"},
	{"Shield", "ShieldComponent"},
	{"Boost", "BoostComponent"},
	{"Weapon", "WeaponComponent"},
	{"Orb", "OrbComponent"},
	{"Ping", "PingComponent"},
	{"CursorView", "CursorViewComponent"},

	// --- Entity Behaviors ---
	{"Decay", "DecayComponent"},
	{"Blossom", "BlossomComponent"},
	{"Cleaner", "CleanerComponent"},
	{"Dust", "DustComponent"},
	{"Navigation", "NavigationComponent"},
	{"Combat", "CombatComponent"},
	{"Genotype", "GenotypeComponent"},
	{"Lightning", "LightningComponent"},
	{"Missile", "MissileComponent"},
	{"Pulse", "PulseComponent"},
	{"Spirit", "SpiritComponent"},
	{"Materialize", "MaterializeComponent"},

	// --- Species ---
	{"Target", "TargetComponent"},
	{"TargetAnchor", "TargetAnchorComponent"},
	{"Drain", "DrainComponent"},
	{"Quasar", "QuasarComponent"},
	{"Swarm", "SwarmComponent"},
	{"Storm", "StormComponent"},
	{"StormCircle", "StormCircleComponent"},
	{"Bullet", "BulletComponent"},
	{"Pylon", "PylonComponent"},
	{"Snake", "SnakeComponent"},
	{"SnakeHead", "SnakeHeadComponent"},
	{"SnakeBody", "SnakeBodyComponent"},
	{"SnakeMember", "SnakeMemberComponent"},
	{"Eye", "EyeComponent"},
	{"Tower", "TowerComponent"},

	// --- Composite ---
	{"Header", "HeaderComponent"},
	{"Member", "MemberComponent"},

	// --- Effects ---
	{"Flash", "FlashComponent"},
	{"Fadeout", "FadeoutComponent"},
	{"Splash", "SplashComponent"},
	{"Marker", "MarkerComponent"},

	// --- Lifecycle ---
	{"Death", "DeathComponent"},
	{"Timer", "TimerComponent"},
}

// Systems is the authoritative system list
// Order determined by priority in parameters
// Generator produces: RegisterSystems(), ActiveSystems()
var Systems = []SystemDef{
	// --- Core / Frame Setup ---
	{"cursor", "NewCursorSystem", engine.SystemShared},
	{"ping", "NewPingSystem", engine.SystemShared},
	{"transient", "NewTransientSystem", engine.SystemPlayer},
	{"camera", "NewCameraSystem", engine.SystemPlayer},

	// --- Player State ---
	{"energy", "NewEnergySystem", engine.SystemShared},
	{"shield", "NewShieldSystem", engine.SystemShared},
	{"heat", "NewHeatSystem", engine.SystemShared},
	{"boost", "NewBoostSystem", engine.SystemShared},
	{"weapon", "NewWeaponSystem", engine.SystemDual},

	// --- Input Processing ---
	{"typing", "NewTypingSystem", engine.SystemDual},

	// --- Composite / Structure ---
	{"composite", "NewCompositeSystem", engine.SystemShared},
	{"wall", "NewWallSystem", engine.SystemDual},
	{"tower", "NewTowerSystem", engine.SystemShared},
	{"gateway", "NewGatewaySystem", engine.SystemShared},

	// --- Entity Behaviors ---
	{"loot", "NewLootSystem", engine.SystemPlayer},
	{"glyph", "NewGlyphSystem", engine.SystemPlayer},
	{"nugget", "NewNuggetSystem", engine.SystemShared},
	{"decay", "NewDecaySystem", engine.SystemPlayer},
	{"blossom", "NewBlossomSystem", engine.SystemPlayer},
	{"gold", "NewGoldSystem", engine.SystemShared},

	// --- Spawning / Materialize ---
	{"materialize", "NewMaterializeSystem", engine.SystemDual},
	{"cleaner", "NewCleanerSystem", engine.SystemDual},
	{"fuse", "NewFuseSystem", engine.SystemPlayer},
	{"spirit", "NewSpiritSystem", engine.SystemDual},

	// --- Projectiles ---
	{"lightning", "NewLightningSystem", engine.SystemPlayer},
	{"missile", "NewMissileSystem", engine.SystemPlayer},

	// --- Movement / Collision ---
	{"navigation", "NewNavigationSystem", engine.SystemDual},
	{"soft_collision", "NewSoftCollisionSystem", engine.SystemDual},

	// --- Combat ---
	{"combat", "NewCombatSystem", engine.SystemDual},

	// --- Species ---
	{"drain", "NewDrainSystem", engine.SystemPlayer},
	{"quasar", "NewQuasarSystem", engine.SystemDual},
	{"swarm", "NewSwarmSystem", engine.SystemDual},
	{"storm", "NewStormSystem", engine.SystemDual},
	{"pylon", "NewPylonSystem", engine.SystemShared},
	{"snake", "NewSnakeSystem", engine.SystemDual},
	{"eye", "NewEyeSystem", engine.SystemDual},
	{"bullet", "NewBulletSystem", engine.SystemPlayer},

	// --- Particles / Effects ---
	{"dust", "NewDustSystem", engine.SystemPlayer},
	{"flash", "NewFlashSystem", engine.SystemPlayer},
	{"fadeout", "NewFadeoutSystem", engine.SystemPlayer},
	{"marker", "NewMarkerSystem", engine.SystemShared},
	{"explosion", "NewExplosionSystem", engine.SystemShared},
	{"motion_marker", "NewMotionMarkerSystem", engine.SystemPlayer},
	{"splash", "NewSplashSystem", engine.SystemPlayer},

	// --- Environment ---
	{"environment", "NewEnvironmentSystem", engine.SystemShared},

	// --- Lifecycle ---
	{"death", "NewDeathSystem", engine.SystemDual},
	{"timekeeper", "NewTimerSystem", engine.SystemDual},
	{"adaptation", "NewAdaptationSystem", engine.SystemShared},
	{"genetic", "NewGeneticSystem", engine.SystemShared},

	// --- Audio ---
	{"audio", "NewAudioSystem", engine.SystemPlayer},
	{"music", "NewMusicSystem", engine.SystemPlayer},
}

// Renderers is the authoritative renderer list
// Order determined by render priority
// Generator produces: RegisterRenderers(), ActiveRenderers()
var Renderers = []RendererDef{
	// --- Background / Grid ---
	{"ping", "NewPingRenderer", "PriorityPing"},
	{"chargeline", "NewChargeLineRenderer", "PriorityChargeLine"},

	// --- Environment ---
	{"wall", "NewWallRenderer", "PriorityWall"},

	// --- Base Entities ---
	{"glyph", "NewGlyphRenderer", "PriorityGlyph"},
	{"sigil", "NewSigilRenderer", "PrioritySigil"},
	{"gold", "NewGoldRenderer", "PriorityGold"},
	{"healthbar", "NewHealthBarRenderer", "PriorityHealthBar"},

	// --- Species (back to front) ---
	{"pylon", "NewPylonRenderer", "PriorityPylon"},
	{"tower", "NewTowerRenderer", "PriorityTower"},
	{"eye", "NewEyeRenderer", "PriorityEye"},
	{"snake", "NewSnakeRenderer", "PrioritySnake"},
	{"drain", "NewDrainRenderer", "PriorityDrain"},
	{"quasar", "NewQuasarRenderer", "PriorityQuasar"},
	{"swarm", "NewSwarmRenderer", "PrioritySwarm"},
	{"storm", "NewStormRenderer", "PriorityStorm"},

	// --- Cleaner ---
	{"cleaner", "NewCleanerRenderer", "PriorityCleaner"},

	// --- Materialize ---
	{"materialize", "NewMaterializeRenderer", "PriorityMaterialize"},
	{"teleportline", "NewTeleportLineRenderer", "PriorityTeleportLine"},

	// --- Field Effects ---
	{"shield", "NewShieldRenderer", "PriorityShield"},
	{"ember", "NewEmberRenderer", "PriorityEmber"},
	{"orb", "NewOrbRenderer", "PriorityOrb"},
	{"lightning", "NewLightningRenderer", "PriorityLightning"},
	{"missile", "NewMissileRenderer", "PriorityMissile"},
	{"pulse", "NewPulseRenderer", "PriorityPulse"},
	{"bullet", "NewBulletRenderer", "PriorityBullet"},

	// --- Particles ---
	{"flash", "NewFlashRenderer", "PriorityFlash"},
	{"fadeout", "NewFadeoutRenderer", "PriorityFadeout"},
	{"explosion", "NewExplosionRenderer", "PriorityExplosion"},
	{"spirit", "NewSpiritRenderer", "PrioritySpirit"},

	// --- Overlays ---
	{"splash", "NewSplashRenderer", "PrioritySplash"},
	{"marker", "NewMarkerRenderer", "PriorityMarker"},

	// --- Post-Processing ---
	{"grayout", "NewGrayoutRenderer", "PriorityGrayout"},
	{"strobe", "NewStrobeRenderer", "PriorityStrobe"},
	{"dim", "NewDimRenderer", "PriorityDim"},

	// --- UI ---
	{"heat", "NewHeatRenderer", "PriorityHeat"},
	{"indicator", "NewIndicatorRenderer", "PriorityIndicator"},
	{"statusbar", "NewStatusBarRenderer", "PriorityStatusBar"},
	{"cursor", "NewCursorRenderer", "PriorityCursor"},

	// --- Debug ---
	{"flowfield", "NewFlowFieldDebugRenderer", "PriorityFlowField"},
	{"pinned_state", "NewPinnedStatsRenderer", "PriorityPinnedState"},
	{"overlay", "NewOverlayRenderer", "PriorityOverlay"},
}
