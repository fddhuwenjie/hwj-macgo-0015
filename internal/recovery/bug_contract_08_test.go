package recovery_test
import ("context"; "testing"; "evidence/internal/journal"; "evidence/internal/recovery")
func TestBug08RecoveryPreservesValidJournal(t *testing.T){dir:=t.TempDir(); path:=dir+"/journal.log"; j,_:=journal.NewJournal(path); _=j.Append([]byte("one")); _=j.Append([]byte("two")); _=j.Close(); if _,err:=recovery.Recover(context.Background(),dir+"/repo",path);err!=nil{t.Fatal(err)}; reopened,_:=journal.NewJournal(path); defer reopened.Close(); records,err:=reopened.Replay(); if err!=nil||len(records)!=2{t.Fatalf("records=%q err=%v",records,err)}}
