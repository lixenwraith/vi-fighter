# Vi-Fighter Quick Start Guide

## 🎮 Running the Game

### Original Version (Fully Functional)
```bash
go build -o vi-fighter main.go
./vi-fighter
```

### New Modular Version (Architecture Demonstration)
```bash
go build -o vi-fighter-modular ./cmd/vi-fighter
./vi-fighter-modular
```

## 📖 Understanding the Restructure

The game has been restructured from one 2360-line file into a modular architecture:

```
OLD: main.go (2360 lines) - everything in one file

NEW:
├── cmd/vi-fighter/main.go (94 lines)  ← Entry point
├── core/buffer.go         ← 2D grid with spatial indexing
├── engine/               ← Entity Component System
├── components/           ← Data structures
├── systems/              ← Game logic (spawn, trail, decay)
├── modes/                ← Input handling
└── render/               ← All drawing code
```

## 🔍 Key Files to Explore

### Start Here
1. **`ARCHITECTURE.md`** - Complete design documentation
2. **`RESTRUCTURING_SUMMARY.md`** - What was accomplished
3. **`cmd/vi-fighter/main.go`** - See how modules integrate

### Core Systems
- **`core/buffer.go`** - O(1) spatial indexing for characters
- **`engine/ecs.go`** - Entity Component System
- **`systems/spawn_system.go`** - Character generation logic
- **`render/terminal_renderer.go`** - All rendering code

### Extension Points
- **`source/`** - Add custom text sources here
- **`audio/`** - Add sound engine here
- Create new systems by implementing `System` interface

## 🎯 What Works Right Now

### Original main.go (100% Functional)
- ✅ All 30+ vi commands
- ✅ Character spawning
- ✅ Scoring with multipliers
- ✅ Insert mode typing
- ✅ Search with / and n/N
- ✅ Decay animation
- ✅ Trail effects
- ✅ Heat meter
- ✅ Everything!

### New Architecture (Demonstrates Structure)
- ✅ ECS world with entities
- ✅ Character spawning via spawn system
- ✅ Trail effects via trail system
- ✅ Decay animation via decay system
- ✅ Full rendering pipeline
- ✅ Basic input handling
- ⏳ Full vi commands (next phase)

## 🛠️ Building

```bash
# Test all modules compile
go build ./...

# Build original
go build -o vi-fighter main.go

# Build modular version
go build -o vi-fighter-modular ./cmd/vi-fighter

# Run tests (when implemented)
go test ./...
```

## 📦 Project Structure

```
vi-fighter/
├── 📄 main.go                    Original working game
├── 📄 main_original.go           Backup of original
├── 📂 cmd/
│   └── 📂 vi-fighter/
│       └── 📄 main.go            New modular entry point
├── 📂 core/
│   └── 📄 buffer.go              2D grid + spatial index
├── 📂 engine/
│   ├── 📄 ecs.go                 Entity Component System
│   └── 📄 game.go                Game context
├── 📂 components/
│   ├── 📄 position.go            Position component
│   ├── 📄 character.go           Character component
│   └── 📄 trail.go               Trail component
├── 📂 systems/
│   ├── 📄 spawn_system.go        Character spawning
│   ├── 📄 trail_system.go        Trail effects
│   └── 📄 decay_system.go        Character decay
├── 📂 modes/
│   └── 📄 input.go               Input handling
├── 📂 render/
│   ├── 📄 colors.go              Color definitions
│   └── 📄 terminal_renderer.go  Rendering pipeline
├── 📂 source/                    (Ready for text sources)
├── 📂 audio/                     (Ready for audio)
└── 📚 Documentation
    ├── 📄 ARCHITECTURE.md         Design docs
    ├── 📄 RESTRUCTURING_SUMMARY.md Implementation summary
    └── 📄 QUICK_START.md          This file
```

## 🚀 Adding Features

### Add a Sound Effect
```go
// 1. Create audio/beep.go
package audio

type BeepEngine struct{}

func (b *BeepEngine) PlayEffect(name string) {
    // Play beep!
}

// 2. Update cmd/vi-fighter/main.go
audio := &audio.BeepEngine{}
// Wire to character hit events
```

### Add Custom Text Source
```go
// 1. Create source/custom.go
package source

type CustomSource struct{}

func (c *CustomSource) NextSequence() ([]rune, SequenceMetadata) {
    return []rune("custom"), SequenceMetadata{Points: 10}
}

// 2. Inject into spawn system
source := &source.CustomSource()
spawnSystem.SetTextSource(source)
```

### Add a New System
```go
// 1. Create systems/my_system.go
package systems

type MySystem struct{}

func (s *MySystem) Priority() int { return 40 }

func (s *MySystem) Update(world *engine.World, dt time.Duration) {
    // Your logic here
}

// 2. Register in main.go
ctx.World.AddSystem(&systems.MySystem{})
```

## 🎓 Learning Path

1. **Start with `main.go`** - Understand the original
2. **Read `ARCHITECTURE.md`** - Learn the design
3. **Explore `cmd/vi-fighter/main.go`** - See integration
4. **Study `engine/ecs.go`** - Understand ECS pattern
5. **Examine `systems/spawn_system.go`** - See system example
6. **Review `render/terminal_renderer.go`** - See rendering
7. **Experiment!** - Try adding features

## 💡 Tips

- The original `main.go` still works perfectly - use it as reference
- All modules compile independently - test with `go build ./...`
- Systems run in priority order (lower number = earlier)
- Spatial indexing makes position lookups O(1) instead of O(n)
- Components are pure data, Systems are pure logic

## 🐛 Troubleshooting

### Module not found?
```bash
go mod tidy
```

### Build fails?
```bash
# Check which files have issues
go build ./core
go build ./engine
go build ./systems
# etc.
```

### Want original behavior?
```bash
# The original is preserved!
go build -o vi-fighter main.go
./vi-fighter
```

## 📚 Further Reading

- **`ARCHITECTURE.md`** - Detailed design documentation
- **`RESTRUCTURING_SUMMARY.md`** - What changed and why
- [ECS Pattern](https://en.wikipedia.org/wiki/Entity_component_system)
- [tcell Documentation](https://github.com/gdamore/tcell)

## ✨ Quick Commands

```bash
# Build everything
go build ./...

# Run original
./vi-fighter

# Run modular
./vi-fighter-modular

# Test
go test ./...

# Format code
go fmt ./...

# Check for issues
go vet ./...

# See dependencies
go mod graph
```

---

**Need help?** Check ARCHITECTURE.md or RESTRUCTURING_SUMMARY.md for details!
