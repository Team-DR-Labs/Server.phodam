package dating

import "github.com/google/uuid"

// ThemeCandidates 는 다음 데이트 테마 후보를 고른다.
// 아직 쓰지 않은 테마가 있으면 그것들을, 모두 썼으면 직전 테마를 뺀 전체를 반환한다.
func ThemeCandidates(all []Theme, used map[uuid.UUID]bool, last *uuid.UUID) []Theme {
	unused := make([]Theme, 0, len(all))
	for _, t := range all {
		if !used[t.ID] {
			unused = append(unused, t)
		}
	}
	if len(unused) > 0 {
		return unused
	}
	rest := make([]Theme, 0, len(all))
	for _, t := range all {
		if last == nil || t.ID != *last {
			rest = append(rest, t)
		}
	}
	if len(rest) == 0 {
		return append([]Theme(nil), all...)
	}
	return rest
}
