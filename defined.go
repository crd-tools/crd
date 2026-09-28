package crd

import (
	"sync"
)

// Реестр определений CRD
//
// Определения добавляются через Add в init пакета: так они гарантированно зарегистрированы
// до запуска main. Инструмент crd generate находит пакеты с вызовами Add в init,
// импортирует их и берёт определения из List

var defined struct {
	mu   sync.Mutex
	list []CRD
	seen map[CRD]bool
}

// Add добавляет определение CRD в реестр
// Повторное добавление того же определения ничего не делает
func Add(c CRD) {
	if c == nil {
		return
	}
	defined.mu.Lock()
	defer defined.mu.Unlock()
	if defined.seen == nil {
		defined.seen = map[CRD]bool{}
	}
	if defined.seen[c] {
		return
	}
	defined.seen[c] = true
	defined.list = append(defined.list, c)
}

// List возвращает определения CRD в порядке добавления
func List() []CRD {
	defined.mu.Lock()
	defer defined.mu.Unlock()
	return append([]CRD(nil), defined.list...)
}
