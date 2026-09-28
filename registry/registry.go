package registry

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"sort"
	"sync"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// ErrNotFound схемы для типа нет в реестре
var ErrNotFound = errors.New("schema not found")

// ErrConflict схемы одного типа не совпадают
var ErrConflict = errors.New("conflicting schemas")

// ErrRecursive схема ссылается сама на себя
var ErrRecursive = errors.New("recursive schema reference")

// ErrModify модификатор схемы вернул ошибку
var ErrModify = errors.New("schema modifier failed")

// ModifyFunc правит копию схемы одного типа перед раскрытием ссылок
type ModifyFunc func(key string, s *apiextv1.JSONSchemaProps) error

// RuntimeVersion версия схем, зарегистрированных из кода
const RuntimeVersion = "runtime"

// Info это сведения о происхождении схемы типа
type Info struct {
	// Key ключ типа
	Key string

	// Package путь импорта пакета
	Package string

	// Name имя типа
	Name string

	// Module модуль пакета, пусто для схем из кода
	Module string

	// Version версия модуля, хэш исходников или RuntimeVersion
	Version string

	// Generator версия генератора, пусто для схем из кода
	Generator string

	// Refs ключи типов, на которые ссылается схема
	Refs []string
}

// Registry это набор схем типов с доступом по ключу
// Схемы из кода имеют приоритет над снимками, в том числе при раскрытии $ref
// Файлы снимков читаются лениво, при первом обращении к любому их типу
type Registry struct {
	mu       sync.Mutex
	runtime  map[string]*apiextv1.JSONSchemaProps
	entries  map[string]*entry
	roots    map[string][]string
	resolved map[string]*apiextv1.JSONSchemaProps
}

// source это один каталог снимков
type source struct {
	fsys   fs.FS
	loaded map[string]bool
}

// entry это схема типа из снимков
type entry struct {
	src    *source
	file   string
	hash   []byte
	info   *Info
	raw    []byte
	schema *apiextv1.JSONSchemaProps
}

// New возвращает пустой реестр
func New() *Registry {
	return &Registry{
		runtime:  map[string]*apiextv1.JSONSchemaProps{},
		entries:  map[string]*entry{},
		roots:    map[string][]string{},
		resolved: map[string]*apiextv1.JSONSchemaProps{},
	}
}

// AddFS подключает каталог снимков, читая только его индекс
func (r *Registry) AddFS(fsys fs.FS) error {
	data, err := fs.ReadFile(fsys, IndexFile)
	if err != nil {
		return err
	}
	idx, err := UnmarshalIndex(data)
	if err != nil {
		return fmt.Errorf("%s: %w", IndexFile, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	src := &source{fsys: fsys, loaded: map[string]bool{}}
	for _, t := range idx.Types {
		if cur, ok := r.entries[t.Type]; ok {
			if !bytes.Equal(cur.hash, t.Hash) {
				return fmt.Errorf("%w for %s", ErrConflict, t.Type)
			}
			continue
		}
		r.entries[t.Type] = &entry{src: src, file: t.File, hash: t.Hash}
	}
	r.addRoots(idx.Roots)
	r.resolved = map[string]*apiextv1.JSONSchemaProps{}
	return nil
}

// Load подключает каталог снимков dir внутри fsys
func (r *Registry) Load(fsys fs.FS, dir string) error {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		return err
	}
	return r.AddFS(sub)
}

// Add добавляет уже прочитанные манифест и файлы пакетов
func (r *Registry) Add(idx Index, files ...File) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range files {
		for _, t := range f.Types {
			key := Key(f.Package, t.Name)
			hash := Hash(t.Schema)
			if cur, ok := r.entries[key]; ok {
				if !bytes.Equal(cur.hash, hash) {
					return fmt.Errorf("%w for %s", ErrConflict, key)
				}
				continue
			}
			r.entries[key] = &entry{hash: hash, info: fileInfo(f, t), raw: t.Schema}
		}
	}
	r.addRoots(idx.Roots)
	r.resolved = map[string]*apiextv1.JSONSchemaProps{}
	return nil
}

// Set регистрирует схему типа из кода, она перекрывает схему из снимков
func (r *Registry) Set(key string, s apiextv1.JSONSchemaProps) error {
	if _, _, err := SplitKey(key); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runtime[key] = s.DeepCopy()
	r.resolved = map[string]*apiextv1.JSONSchemaProps{}
	return nil
}

// Has сообщает, есть ли схема типа в снимках или среди схем из кода
func (r *Registry) Has(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runtime[key]; ok {
		return true
	}
	_, ok := r.entries[key]
	return ok
}

// HasRuntime сообщает, зарегистрирована ли схема типа из кода
func (r *Registry) HasRuntime(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.runtime[key]
	return ok
}

func (r *Registry) addRoots(roots []Root) {
	for _, root := range roots {
		r.roots[root.Type] = uniqSorted(append(r.roots[root.Type], root.Usages...))
	}
}

// Keys возвращает отсортированные ключи всех типов
func (r *Registry) Keys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	set := map[string]struct{}{}
	for k := range r.entries {
		set[k] = struct{}{}
	}
	for k := range r.runtime {
		set[k] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Roots возвращает типы, запрошенные метками, с местами использования
func (r *Registry) Roots() []Root {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Root, 0, len(r.roots))
	for k, usages := range r.roots {
		out = append(out, Root{Type: k, Usages: append([]string(nil), usages...)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// Info возвращает сведения о происхождении схемы
//
//	ErrNotFound
func (r *Registry) Info(key string) (Info, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.runtime[key]; ok {
		pkg, name, _ := SplitKey(key)
		return Info{Key: key, Package: pkg, Name: name, Version: RuntimeVersion, Refs: Refs(s)}, nil
	}
	e, ok := r.entries[key]
	if !ok {
		return Info{}, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err := r.loadEntry(key, e); err != nil {
		return Info{}, err
	}
	info := *e.info
	info.Refs = append([]string(nil), info.Refs...)
	return info, nil
}

// Get возвращает копию схемы типа как она хранится, со ссылками $ref
//
//	ErrNotFound
func (r *Registry) Get(key string) (*apiextv1.JSONSchemaProps, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, err := r.decode(key)
	if err != nil {
		return nil, err
	}
	return s.DeepCopy(), nil
}

// Resolve возвращает копию схемы типа с раскрытыми ссылками и слитым allOf
//
//	ErrNotFound
func (r *Registry) Resolve(key string) (*apiextv1.JSONSchemaProps, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, err := r.resolve(key, map[string]bool{})
	if err != nil {
		return nil, err
	}
	return s.DeepCopy(), nil
}

// For возвращает раскрытую схему для Go-типа
//
//	ErrNotFound
func (r *Registry) For(t reflect.Type) (*apiextv1.JSONSchemaProps, error) {
	key, ok := KeyOf(t)
	if !ok {
		return nil, fmt.Errorf("type %s has no registry key", t)
	}
	return r.Resolve(key)
}

// KeyOf возвращает ключ реестра для именованного Go-типа
func KeyOf(t reflect.Type) (string, bool) {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Name() == "" || t.PkgPath() == "" {
		return "", false
	}
	return Key(t.PkgPath(), t.Name()), true
}

// decode возвращает схему типа, схемы из кода важнее снимков
func (r *Registry) decode(key string) (*apiextv1.JSONSchemaProps, error) {
	if s, ok := r.runtime[key]; ok {
		return s, nil
	}
	e, ok := r.entries[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if e.schema != nil {
		return e.schema, nil
	}
	if err := r.loadEntry(key, e); err != nil {
		return nil, err
	}
	s := &apiextv1.JSONSchemaProps{}
	if err := s.Unmarshal(e.raw); err != nil {
		return nil, fmt.Errorf("decode %s: %w", key, err)
	}
	e.schema = s
	e.raw = nil
	return s, nil
}

// loadEntry читает файл снимка, в котором лежит тип, если он ещё не прочитан
func (r *Registry) loadEntry(key string, e *entry) error {
	if e.info != nil {
		return nil
	}
	if e.src == nil || e.src.loaded[e.file] {
		return fmt.Errorf("%s: missing in %s", key, e.file)
	}
	data, err := fs.ReadFile(e.src.fsys, e.file)
	if err != nil {
		return err
	}
	f, err := UnmarshalFile(data)
	if err != nil {
		return fmt.Errorf("%s: %w", e.file, err)
	}
	e.src.loaded[e.file] = true
	for _, t := range f.Types {
		k := Key(f.Package, t.Name)
		other, ok := r.entries[k]
		if !ok || other.src != e.src || other.file != e.file {
			continue
		}
		if !bytes.Equal(other.hash, Hash(t.Schema)) {
			return fmt.Errorf("%s: schema of %s does not match the index", e.file, k)
		}
		other.info = fileInfo(f, t)
		other.raw = t.Schema
	}
	if e.info == nil {
		return fmt.Errorf("%s: missing in %s", key, e.file)
	}
	return nil
}

func fileInfo(f File, t Type) *Info {
	return &Info{
		Key:       Key(f.Package, t.Name),
		Package:   f.Package,
		Name:      t.Name,
		Module:    f.Module,
		Version:   f.Version,
		Generator: f.Generator,
		Refs:      append([]string(nil), t.Refs...),
	}
}

// ResolveWith раскрывает схему, применяя modify к схеме каждого типа до раскрытия ссылок
// Результат не попадает в общий кеш: разные модификаторы дают независимые результаты
//
//	ErrNotFound
//	ErrModify
func (r *Registry) ResolveWith(key string, modify ModifyFunc) (*apiextv1.JSONSchemaProps, error) {
	if modify == nil {
		return r.Resolve(key)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	pass := &resolvePass{resolved: map[string]*apiextv1.JSONSchemaProps{}, modify: modify}
	s, err := r.resolveIn(pass, key, map[string]bool{})
	if err != nil {
		return nil, err
	}
	return s.DeepCopy(), nil
}

// Modified возвращает копию схемы типа как она хранится, со ссылками, после modify
//
//	ErrNotFound
//	ErrModify
func (r *Registry) Modified(key string, modify ModifyFunc) (*apiextv1.JSONSchemaProps, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	raw, err := r.decode(key)
	if err != nil {
		return nil, err
	}
	s := raw.DeepCopy()
	if modify != nil {
		if err := modify(key, s); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrModify, key, err)
		}
	}
	return s, nil
}

// resolvePass это один проход раскрытия со своим кешем и модификатором
type resolvePass struct {
	resolved map[string]*apiextv1.JSONSchemaProps
	modify   ModifyFunc
}

func (r *Registry) resolve(key string, stack map[string]bool) (*apiextv1.JSONSchemaProps, error) {
	return r.resolveIn(&resolvePass{resolved: r.resolved}, key, stack)
}

func (r *Registry) resolveIn(pass *resolvePass, key string, stack map[string]bool) (*apiextv1.JSONSchemaProps, error) {
	if s, ok := pass.resolved[key]; ok {
		return s, nil
	}
	if stack[key] {
		return nil, fmt.Errorf("%w: %s", ErrRecursive, key)
	}
	stack[key] = true
	defer delete(stack, key)

	raw, err := r.decode(key)
	if err != nil {
		return nil, err
	}
	s := raw.DeepCopy()
	if pass.modify != nil {
		if err := pass.modify(key, s); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrModify, key, err)
		}
	}

	var walkErr error
	Walk(s, func(node *apiextv1.JSONSchemaProps) bool {
		if walkErr != nil {
			return false
		}
		if node.Ref == nil || *node.Ref == "" {
			return true
		}
		ref := *node.Ref
		target, err := r.resolveIn(pass, ref, stack)
		if err != nil {
			walkErr = fmt.Errorf("%s: %w", key, err)
			return false
		}
		merged, err := overlay(*target.DeepCopy(), *node)
		if err != nil {
			walkErr = fmt.Errorf("%s -> %s: %w", key, ref, err)
			return false
		}
		*node = merged
		return false
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if err := flattenAllOf(s); err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	pass.resolved[key] = s
	return s, nil
}
