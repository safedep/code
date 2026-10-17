using Web;

var builder = WebApplication.CreateBuilder(args);
var app = builder.Build();
app.MapGet("/health", Health.Check);
app.Run();
