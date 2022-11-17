package concurrency

import (
	"reflect"
	"sort"
	"testing"
)

func TestDispatcher(t *testing.T) {
	const ceiling = 100
	const concurrency = 10
	input := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	multipliers := []int{2, 3, 5, 7}
	var expected, got []int

	work := func(n int) []int {
		var output []int
		for _, m := range multipliers {
			output = append(output, (n*m)%ceiling)
		}
		return output
	}

	seen := map[int]bool{}
	var queue []int
	for _, i := range input {
		queue = append(queue, i)
	}
	for len(queue) > 0 {
		output := work(queue[0])
		queue = queue[1:]
		for _, n := range output {
			if !seen[n] {
				seen[n] = true
				expected = append(expected, n)
				queue = append(queue, n)
			}
		}
	}
	sort.Ints(expected)

	seen = map[int]bool{}
	items := []*Item[int, []int]{{Output: input}}
	Dispatcher(
		func(i *Item[int, []int]) ([]*Item[int, []int], error) {
			var disp []*Item[int, []int]
			for _, n := range i.Output {
				if !seen[n] {
					seen[n] = true
					disp = append(disp, &Item[int, []int]{
						Data:   n,
						Output: nil,
						Err:    nil,
					})
					t.Logf("work in=%d out=%d", i.Data, n)
				} else {
					t.Logf("skip in=%d out=%d", i.Data, n)
				}
			}
			return disp, nil
		},
		func(i *Item[int, []int]) ([]int, error) { return work(i.Data), nil },
		items,
		concurrency,
	)
	t.Log(seen)

	for n := range seen {
		got = append(got, n)
	}
	sort.Ints(got)

	if !reflect.DeepEqual(expected, got) {
		t.Errorf("Dispatcher() = %v, want %v", got, expected)
	}
}
