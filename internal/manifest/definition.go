package manifest

import "github.com/lixenwraith/vi-fighter/internal/engine"

// ComponentDef defines a component for registration and store generation
type ComponentDef struct {
	Field string // ComponentStore field name (e.g., "Drain")
	Type  string // Type name without package (e.g., "DrainComponent")
}

// SystemDef declares one system's identity, domain and construction edges.
type SystemDef struct {
	Name        string // Registry key (e.g., "drain")
	Constructor string // Constructor name without package (e.g., "NewDrainSystem")
	Domain      engine.SystemDomain
	Required    []string // Must be registered and enabled.
	Optional    []string // Orders construction when present.
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

// Systems is the authoritative system profile list.
// Slice order breaks equal-priority tick ties; dependencies order construction.
var Systems = []SystemDef{
	// --- Core / Frame Setup ---
	{"cursor", "NewCursorSystem", engine.SystemShared, nil, nil},
	{"ping", "NewPingSystem", engine.SystemShared, []string{"cursor"}, nil},
	{"transient", "NewTransientSystem", engine.SystemPlayer, nil, nil},
	{"camera", "NewCameraSystem", engine.SystemPlayer, []string{"cursor"}, nil},

	// --- Player State ---
	{"energy", "NewEnergySystem", engine.SystemShared, []string{"cursor"}, nil},
	{"shield", "NewShieldSystem", engine.SystemShared, []string{"cursor", "energy"}, nil},
	{"heat", "NewHeatSystem", engine.SystemShared, []string{"cursor"}, nil},
	{"boost", "NewBoostSystem", engine.SystemShared, []string{"cursor"}, nil},
	{"weapon", "NewWeaponSystem", engine.SystemDual, []string{"cursor"}, []string{"combat", "cleaner", "missile", "energy"}},

	// --- Input Processing ---
	{"typing", "NewTypingSystem", engine.SystemDual, []string{"cursor"}, []string{"glyph", "death", "energy", "heat", "boost", "composite", "audio"}},

	// --- Composite / Structure ---
	{"composite", "NewCompositeSystem", engine.SystemShared, []string{"death"}, nil},
	{"wall", "NewWallSystem", engine.SystemDual, nil, []string{"cursor", "death", "fadeout"}},
	{"tower", "NewTowerSystem", engine.SystemShared, []string{"composite", "combat", "death"}, []string{"navigation"}},
	{"gateway", "NewGatewaySystem", engine.SystemShared, nil, []string{"navigation", "eye", "snake"}},

	// --- Entity Behaviors ---
	{"loot", "NewLootSystem", engine.SystemPlayer, []string{"cursor"}, []string{"death", "energy", "heat", "weapon", "flash"}},
	{"glyph", "NewGlyphSystem", engine.SystemPlayer, nil, []string{"wall"}},
	{"nugget", "NewNuggetSystem", engine.SystemShared, nil, []string{"cursor", "cleaner", "energy", "heat", "audio"}},
	{"decay", "NewDecaySystem", engine.SystemPlayer, []string{"death"}, []string{"glyph", "flash", "wall"}},
	{"blossom", "NewBlossomSystem", engine.SystemPlayer, []string{"death"}, []string{"glyph", "wall"}},
	{"gold", "NewGoldSystem", engine.SystemShared, []string{"cursor", "composite"}, []string{"energy", "splash", "audio", "death"}},

	// --- Spawning / Materialize ---
	{"materialize", "NewMaterializeSystem", engine.SystemDual, nil, nil},
	{"cleaner", "NewCleanerSystem", engine.SystemDual, []string{"cursor"}, []string{"combat", "decay", "blossom", "audio"}},
	{"fuse", "NewFuseSystem", engine.SystemPlayer, []string{"drain"}, []string{"materialize", "spirit", "quasar", "swarm"}},
	{"spirit", "NewSpiritSystem", engine.SystemDual, nil, nil},

	// --- Projectiles ---
	{"lightning", "NewLightningSystem", engine.SystemPlayer, nil, nil},
	{"missile", "NewMissileSystem", engine.SystemPlayer, []string{"explosion"}, []string{"combat", "wall"}},

	// --- Movement / Collision ---
	{"navigation", "NewNavigationSystem", engine.SystemDual, nil, []string{"cursor", "wall"}},
	{"soft_collision", "NewSoftCollisionSystem", engine.SystemDual, nil, []string{"drain", "quasar", "swarm", "storm", "pylon"}},

	// --- Combat ---
	{"combat", "NewCombatSystem", engine.SystemDual, nil, []string{"cursor", "energy", "lightning"}},

	// --- Species ---
	{"drain", "NewDrainSystem", engine.SystemPlayer, []string{"cursor"}, []string{"combat", "materialize", "death", "flash", "dust"}},
	{"quasar", "NewQuasarSystem", engine.SystemDual, []string{"composite", "combat", "death"}, []string{"navigation", "lightning", "splash", "flash"}},
	{"swarm", "NewSwarmSystem", engine.SystemDual, []string{"composite", "combat", "death"}, []string{"navigation"}},
	{"storm", "NewStormSystem", engine.SystemDual, []string{"composite", "combat", "death"}, []string{"navigation", "bullet", "materialize", "flash"}},
	{"pylon", "NewPylonSystem", engine.SystemShared, []string{"composite", "combat", "death"}, []string{"navigation"}},
	{"snake", "NewSnakeSystem", engine.SystemDual, []string{"composite", "combat", "death"}, []string{"navigation", "flash"}},
	{"eye", "NewEyeSystem", engine.SystemDual, []string{"composite", "combat", "death"}, []string{"navigation", "explosion"}},
	{"bullet", "NewBulletSystem", engine.SystemPlayer, []string{"cursor"}, []string{"shield", "heat"}},

	// --- Particles / Effects ---
	{"dust", "NewDustSystem", engine.SystemPlayer, []string{"explosion"}, []string{"flash", "death"}},
	{"flash", "NewFlashSystem", engine.SystemPlayer, nil, nil},
	{"fadeout", "NewFadeoutSystem", engine.SystemPlayer, nil, nil},
	{"marker", "NewMarkerSystem", engine.SystemShared, nil, nil},
	{"explosion", "NewExplosionSystem", engine.SystemShared, nil, []string{"combat"}},
	{"motion_marker", "NewMotionMarkerSystem", engine.SystemPlayer, []string{"cursor"}, []string{"glyph"}},
	{"splash", "NewSplashSystem", engine.SystemPlayer, []string{"cursor"}, nil},

	// --- Environment ---
	{"environment", "NewEnvironmentSystem", engine.SystemShared, nil, nil},

	// --- Lifecycle ---
	{"death", "NewDeathSystem", engine.SystemDual, nil, nil},
	{"timekeeper", "NewTimerSystem", engine.SystemDual, []string{"death"}, nil},
	{"adaptation", "NewAdaptationSystem", engine.SystemShared, []string{"navigation"}, []string{"gateway"}},
	{"genetic", "NewGeneticSystem", engine.SystemShared, nil, []string{"combat", "eye"}},

	// --- Audio ---
	{"audio", "NewAudioSystem", engine.SystemPlayer, nil, nil},
	{"music", "NewMusicSystem", engine.SystemPlayer, nil, []string{"audio"}},
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
