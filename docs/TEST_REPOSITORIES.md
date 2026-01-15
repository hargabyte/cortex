# Test Repositories for Cortex Language Support

This document lists small, beginner-friendly GitHub repositories for testing Cortex's language support. Each repository is chosen for being:
- Small enough to scan quickly
- Representative of real-world code patterns
- Well-structured with clear entity definitions

## Quick Setup Script

```bash
# Create test directory
mkdir -p ~/cortex-test-repos && cd ~/cortex-test-repos

# Clone all repositories (run this script or pick individual ones)
# See sections below for individual clone commands
```

---

## Epic 1: Call Graph Completion (Existing Languages)

### TypeScript/JavaScript

**Recommended: simple-typescript-starter**
```bash
git clone https://github.com/stemmlerjs/simple-typescript-starter.git ts-simple
cd ts-simple && npm install && cd ..
```
- Small, clean structure
- Has functions calling other functions
- Good for testing basic call graph

**Alternative: beginners-typescript-tutorial**
```bash
git clone https://github.com/total-typescript/beginners-typescript-tutorial.git ts-tutorial
```
- Multiple exercise files
- Various TypeScript patterns
- Source: [total-typescript/beginners-typescript-tutorial](https://github.com/total-typescript/beginners-typescript-tutorial)

**Larger test: LearningTypeScript projects**
```bash
git clone https://github.com/LearningTypeScript/projects.git ts-projects
```
- Real-world project examples
- Good for integration testing
- Source: [LearningTypeScript/projects](https://github.com/LearningTypeScript/projects)

---

### Python

**Recommended: python-mini-projects**
```bash
git clone https://github.com/Python-World/python-mini-projects.git python-mini
```
- Collection of small projects
- Various patterns (CLI, games, utilities)
- Source: [Python-World/python-mini-projects](https://github.com/Python-World/python-mini-projects)

**Alternative: python-beginner-projects**
```bash
git clone https://github.com/Mrinank-Bhowmick/python-beginner-projects.git python-beginner
```
- Minimal code per project
- Good for isolating specific patterns
- Source: [Mrinank-Bhowmick/python-beginner-projects](https://github.com/Mrinank-Bhowmick/python-beginner-projects)

**With classes/OOP: All-In-One-Python-Projects**
```bash
git clone https://github.com/king04aman/All-In-One-Python-Projects.git python-all-in-one
```
- More complex patterns
- Class hierarchies, decorators
- Source: [king04aman/All-In-One-Python-Projects](https://github.com/king04aman/All-In-One-Python-Projects)

---

### Java

**Recommended: simple-java-maven-app**
```bash
git clone https://github.com/jenkins-docs/simple-java-maven-app.git java-maven-simple
```
- Clean Maven structure
- Has unit tests
- Source: [jenkins-docs/simple-java-maven-app](https://github.com/jenkins-docs/simple-java-maven-app)

**Alternative: java-hello-world-with-gradle**
```bash
git clone https://github.com/jabedhasan21/java-hello-world-with-gradle.git java-gradle-hello
```
- Gradle build system
- Minimal but functional
- Source: [jabedhasan21/java-hello-world-with-gradle](https://github.com/jabedhasan21/java-hello-world-with-gradle)

**Larger test: MavenIn28Minutes**
```bash
git clone https://github.com/in28minutes/MavenIn28Minutes.git java-maven-tutorial
```
- Tutorial structure with multiple examples
- More classes and dependencies
- Source: [in28minutes/MavenIn28Minutes](https://github.com/in28minutes/MavenIn28Minutes)

---

### Rust

**Recommended: example_project_structure**
```bash
git clone https://github.com/Rust-Trends/example_project_structure.git rust-structure
```
- Shows standard Rust project layout
- lib.rs + main.rs + modules
- Source: [Rust-Trends/example_project_structure](https://github.com/Rust-Trends/example_project_structure)

**Alternative: Create a fresh Rust project**
```bash
cargo new rust-test-project
cd rust-test-project
# Add some code with function calls, structs, impl blocks
```

**For reference: awesome-rust**
```bash
# Don't clone - just browse for small projects
# https://github.com/rust-unofficial/awesome-rust
```
- Curated list of Rust projects
- Pick small ones for testing
- Source: [rust-unofficial/awesome-rust](https://github.com/rust-unofficial/awesome-rust)

---

## Epic 2: New Languages

### C

**Recommended: Learning-C**
```bash
git clone https://github.com/h0mbre/Learning-C.git c-learning
```
- Mini-projects for beginners
- Functions, structs, headers
- Source: [h0mbre/Learning-C](https://github.com/h0mbre/Learning-C)

**Alternative: Basic-Projects-in-C-Programming**
```bash
git clone https://github.com/ManyamSanjayKumarReddy/Basic-Projects-in-C-Programming.git c-basic
```
- Calculator, hangman, library system
- Good variety of patterns
- Source: [ManyamSanjayKumarReddy/Basic-Projects-in-C-Programming](https://github.com/ManyamSanjayKumarReddy/Basic-Projects-in-C-Programming)

**Project-based tutorials**
```bash
git clone https://github.com/nCally/Project-Based-Tutorials-in-C.git c-tutorials
```
- Links to various C project tutorials
- Source: [nCally/Project-Based-Tutorials-in-C](https://github.com/nCally/Project-Based-Tutorials-in-C)

---

### C++

**Recommended: cmake-project-template**
```bash
git clone https://github.com/kigster/cmake-project-template.git cpp-cmake-template
```
- Minimal CMake setup
- Library + executable + tests
- Source: [kigster/cmake-project-template](https://github.com/kigster/cmake-project-template)

**Tutorial examples: cmake-examples**
```bash
git clone https://github.com/ttroy50/cmake-examples.git cpp-cmake-examples
```
- Progressive complexity
- Many small examples
- Source: [ttroy50/cmake-examples](https://github.com/ttroy50/cmake-examples)

**Modern C++: ModernCppStarter**
```bash
git clone https://github.com/TheLartians/ModernCppStarter.git cpp-modern-starter
```
- Modern C++ practices
- Templates, namespaces, etc.
- Source: [TheLartians/ModernCppStarter](https://github.com/TheLartians/ModernCppStarter)

---

### C#

**Recommended: csharp-for-everybody**
```bash
git clone https://github.com/Kalutu/csharp-for-everybody.git csharp-beginner
```
- Calculator, Tic Tac Toe, etc.
- Good OOP examples
- Source: [Kalutu/csharp-for-everybody](https://github.com/Kalutu/csharp-for-everybody)

**Official samples: dotnet/samples**
```bash
git clone --depth 1 https://github.com/dotnet/samples.git dotnet-samples
# Note: Large repo, use --depth 1
```
- Official .NET samples
- Many small projects
- Source: [dotnet/samples](https://github.com/dotnet/samples)

**ASP.NET examples: practical-aspnetcore**
```bash
git clone https://github.com/dodyg/practical-aspnetcore.git aspnet-practical
```
- Web application patterns
- Source: [dodyg/practical-aspnetcore](https://github.com/dodyg/practical-aspnetcore)

---

### PHP

**Recommended: quickstart-basic (Laravel)**
```bash
git clone https://github.com/laravel/quickstart-basic.git php-laravel-quickstart
cd php-laravel-quickstart && composer install && cd ..
```
- Official Laravel sample
- Task list application
- Source: [laravel/quickstart-basic](https://github.com/laravel/quickstart-basic)

**Tutorial: laravel-basics**
```bash
git clone https://github.com/SagarMaheshwary/laravel-basics.git php-laravel-basics
```
- Laravel + Bootstrap example
- Source: [SagarMaheshwary/laravel-basics](https://github.com/SagarMaheshwary/laravel-basics)

**Pure PHP (no framework): Create simple files**
```bash
mkdir php-simple && cd php-simple
# Create test.php with classes, functions, etc.
```

---

### Kotlin

**Recommended: kotlin-android-practice**
```bash
git clone https://github.com/hieuwu/kotlin-android-practice.git kotlin-android-practice
```
- Small Android projects
- Basic Kotlin patterns
- Source: [hieuwu/kotlin-android-practice](https://github.com/hieuwu/kotlin-android-practice)

**Beginner projects: Android-Beginner-Projects**
```bash
git clone https://github.com/akebu6/Android-Beginner-Projects.git kotlin-beginner
```
- Tic Tac Toe, Calculator, etc.
- Source: [akebu6/Android-Beginner-Projects](https://github.com/akebu6/Android-Beginner-Projects)

**Official examples: kotlin-examples**
```bash
git clone https://github.com/Kotlin/kotlin-examples.git kotlin-official
```
- Note: May be archived, check status
- Source: [Kotlin/kotlin-examples](https://github.com/Kotlin/kotlin-examples)

**25 projects: kotlin-projects**
```bash
git clone https://github.com/solygambas/kotlin-projects.git kotlin-25-projects
```
- Todo, timer, encryption tools
- Source: [solygambas/kotlin-projects](https://github.com/solygambas/kotlin-projects)

---

### Ruby

**Recommended: learn-rails**
```bash
git clone https://github.com/RailsApps/learn-rails.git ruby-learn-rails
cd ruby-learn-rails && bundle install && cd ..
```
- Accompanies "Learn Ruby on Rails" book
- Small, well-documented
- Source: [RailsApps/learn-rails](https://github.com/RailsApps/learn-rails)

**Tutorial: sample_app_6th_ed**
```bash
git clone https://github.com/learnenough/sample_app_6th_ed.git ruby-sample-app
```
- From Rails Tutorial book
- Complete application
- Source: [learnenough/sample_app_6th_ed](https://github.com/learnenough/sample_app_6th_ed)

**Real-world example: rails-realworld-example-app**
```bash
git clone https://github.com/gothinkster/rails-realworld-example-app.git ruby-realworld
```
- JWT auth, API patterns
- Source: [gothinkster/rails-realworld-example-app](https://github.com/gothinkster/rails-realworld-example-app)

---

## All-In-One Clone Script

```bash
#!/bin/bash
# Clone all recommended test repositories

mkdir -p ~/cortex-test-repos && cd ~/cortex-test-repos

echo "=== Epic 1: Existing Languages ==="

# TypeScript
git clone https://github.com/stemmlerjs/simple-typescript-starter.git ts-simple

# Python
git clone https://github.com/Python-World/python-mini-projects.git python-mini

# Java
git clone https://github.com/jenkins-docs/simple-java-maven-app.git java-maven-simple

# Rust
git clone https://github.com/Rust-Trends/example_project_structure.git rust-structure

echo "=== Epic 2: New Languages ==="

# C
git clone https://github.com/h0mbre/Learning-C.git c-learning

# C++
git clone https://github.com/kigster/cmake-project-template.git cpp-cmake-template

# C#
git clone https://github.com/Kalutu/csharp-for-everybody.git csharp-beginner

# PHP
git clone https://github.com/laravel/quickstart-basic.git php-laravel-quickstart

# Kotlin
git clone https://github.com/hieuwu/kotlin-android-practice.git kotlin-android-practice

# Ruby
git clone https://github.com/RailsApps/learn-rails.git ruby-learn-rails

echo "=== Done! ==="
ls -la
```

---

## Testing Workflow

After cloning repositories:

```bash
cd ~/cortex-test-repos

# Test TypeScript
cx scan ts-simple/
cx find --lang typescript
cx impact ts-simple/src/index.ts  # Should work after call graph implemented

# Test Python
cx scan python-mini/
cx find --lang python
cx impact python-mini/projects/...

# Test each language similarly...

# Full scan of all repos
cx scan .
cx db info  # Check entity counts
cx find --important  # See what entities exist
```

---

## Notes

- Some repos may need dependency installation (npm install, composer install, bundle install)
- Large repos like `dotnet/samples` should use `--depth 1` for shallow clone
- Focus on `src/` or main code directories for cleaner scans
- Use `cx scan --exclude node_modules,vendor,bundle` to skip dependencies
