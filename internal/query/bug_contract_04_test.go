package query_test
import ("context"; "testing"; "time"; "evidence/internal/domain"; "evidence/internal/query"; "evidence/internal/repository")
func TestBug04EffectiveQueryUsesRequestedFuture(t *testing.T){ctx:=context.Background(); repo,_:=repository.NewFileRepository(t.TempDir()); now:=time.Now(); exp:=now.Add(time.Hour); _=repo.SaveRequest(ctx,domain.AuthorizationRequest{ID:"r",Status:domain.StatusEnabled,ExpiresAt:&exp}); got,err:=query.NewQueryBuilder(repo).BuildEffectiveAt(ctx,now.Add(2*time.Hour)); if err!=nil||len(got)!=0{t.Fatalf("future result: %#v %v",got,err)}}
