// ASP.NET Core audit-trail example.
//
// Every HTTP request is checkpointed as a tamper-evident EVENT on the
// service quid's stream. Run:
//
//   cd clients/dotnet/examples/AspNetAudit
//   dotnet run

using Quidnug.Client;

var builder = WebApplication.CreateBuilder(args);

// Singletons — shared across all requests.
string nodeUrl = Environment.GetEnvironmentVariable("QUIDNUG_NODE") ?? "http://localhost:8080";
builder.Services.AddSingleton(new QuidnugClient(nodeUrl));
builder.Services.AddSingleton<Quid>(_ => Quid.Generate()); // prod: load from HSM

var app = builder.Build();

// Register the service identity on first start.
var client = app.Services.GetRequiredService<QuidnugClient>();
var service = app.Services.GetRequiredService<Quid>();
try
{
    await client.RegisterIdentityAsync(service,
        name: "audit-service",
        homeDomain: "corp.audit");
    app.Logger.LogInformation("registered service quid {Id}", service.Id);
}
catch (QuidnugException ex)
{
    app.Logger.LogWarning("identity registration: {Msg}", ex.Message);
}

// Middleware: log every request onto the service's event stream.
app.Use(async (ctx, next) =>
{
    await next();

    try
    {
        await client.EmitEventAsync(service,
            subjectId: service.Id,
            subjectType: "QUID",
            eventType: "http.request",
            domain: "corp.audit",
            payload: new Dictionary<string, object?>
            {
                ["method"] = ctx.Request.Method,
                ["path"] = ctx.Request.Path.Value,
                ["status"] = ctx.Response.StatusCode,
                ["userAgent"] = ctx.Request.Headers.UserAgent.ToString(),
                ["remoteIp"] = ctx.Connection.RemoteIpAddress?.ToString(),
            });
    }
    catch (QuidnugException ex)
    {
        app.Logger.LogWarning("audit event failed: {Msg}", ex.Message);
    }
});

app.MapGet("/", () => "Hello — every request is audited into Quidnug.");
app.MapGet("/audit", async () =>
{
    var events = await client.GetStreamEventsAsync(service.Id, "corp.audit", limit: 50);
    return events.Select(e => new { e.Sequence, e.EventType, e.Timestamp, e.Payload });
});

app.Run();
