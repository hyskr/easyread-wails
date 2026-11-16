package chat

import "testing"

func TestPassageRankingChineseAndDiversity(t *testing.T) {
	rows := []passage{{M{"paper": "a", "anchor": "a1", "text": "机器学习研究"}, searchTerms("机器学习研究")}, {M{"paper": "a", "anchor": "a2"}, searchTerms("机器学习")}, {M{"paper": "b", "anchor": "b1"}, searchTerms("机器学习的方法")}}
	got := rankPassages(rows, "机器学习")
	if len(got) != 3 || obj(got[0])["paper"] == obj(got[1])["paper"] {
		t.Fatalf("%v", got)
	}
	if len(rankPassages(rows, "unrelated")) != 0 {
		t.Fatal("unrelated result")
	}
}
