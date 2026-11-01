// internal/domain/loop_mode.go
package domain

// LoopMode 재생 루프 모드
type LoopMode string

const (
	// LoopNone 반복 없음 (마지막 곡 후 정지)
	LoopNone LoopMode = "none"

	// LoopAll 전체 반복
	LoopAll LoopMode = "all"

	// LoopSingle 한 곡 반복
	LoopSingle LoopMode = "single"
)

// Next 버튼 토글 순서: none → all → single → none
func (m LoopMode) Next() LoopMode {
	switch m {
	case LoopNone:
		return LoopAll
	case LoopAll:
		return LoopSingle
	default:
		return LoopNone
	}
}

func (m LoopMode) Valid() bool {
	switch m {
	case LoopNone, LoopAll, LoopSingle:
		return true
	default:
		return false
	}
}
