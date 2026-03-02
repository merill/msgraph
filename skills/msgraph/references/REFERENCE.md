# Microsoft Graph API Reference

This reference covers common Graph API patterns, resources, and conventions that agents should know when constructing API calls.

## Base URL

```
https://graph.microsoft.com/{version}/{resource}
```

- **beta** (default): `https://graph.microsoft.com/beta/`
- **v1.0** (stable): `https://graph.microsoft.com/v1.0/`

## Common Resources

### Identity & Directory

| Resource | Path | Common Operations |
|---|---|---|
| Current user | `/me` | GET profile, PATCH update |
| Users | `/users` | GET list, POST create |
| Specific user | `/users/{id or UPN}` | GET, PATCH, DELETE |
| Groups | `/groups` | GET list, POST create |
| Group members | `/groups/{id}/members` | GET list |
| Service principals | `/servicePrincipals` | GET list |
| Applications | `/applications` | GET list, POST create |
| Directory roles | `/directoryRoles` | GET list |
| Domains | `/domains` | GET list |
| Organization | `/organization` | GET details |

### Mail

| Resource | Path | Common Operations |
|---|---|---|
| Messages | `/me/messages` | GET list |
| Specific message | `/me/messages/{id}` | GET, PATCH, DELETE |
| Send mail | `/me/sendMail` | POST |
| Mail folders | `/me/mailFolders` | GET list |
| Folder messages | `/me/mailFolders/{id}/messages` | GET list |

### Calendar

| Resource | Path | Common Operations |
|---|---|---|
| Events | `/me/events` | GET list, POST create |
| Specific event | `/me/events/{id}` | GET, PATCH, DELETE |
| Calendar view | `/me/calendarView` | GET (requires startDateTime, endDateTime) |
| Calendars | `/me/calendars` | GET list |

### Teams

| Resource | Path | Common Operations |
|---|---|---|
| Joined teams | `/me/joinedTeams` | GET list |
| Team details | `/teams/{id}` | GET |
| Channels | `/teams/{id}/channels` | GET list, POST create |
| Channel messages | `/teams/{id}/channels/{id}/messages` | GET list, POST create |

### SharePoint & OneDrive

| Resource | Path | Common Operations |
|---|---|---|
| My drive | `/me/drive` | GET |
| Drive items | `/me/drive/root/children` | GET list |
| Specific item | `/me/drive/items/{id}` | GET, PATCH, DELETE |
| SharePoint sites | `/sites` | GET list |
| Site by path | `/sites/{hostname}:/{path}` | GET |
| Site lists | `/sites/{id}/lists` | GET list |

### Intune / Device Management

| Resource | Path | Common Operations |
|---|---|---|
| Managed devices | `/deviceManagement/managedDevices` | GET list |
| Compliance policies | `/deviceManagement/deviceCompliancePolicies` | GET list |
| Config profiles | `/deviceManagement/deviceConfigurations` | GET list |
| Mobile apps | `/deviceAppManagement/mobileApps` | GET list |

### Security

| Resource | Path | Common Operations |
|---|---|---|
| Alerts | `/security/alerts_v2` | GET list |
| Incidents | `/security/incidents` | GET list |
| Secure score | `/security/secureScores` | GET list |

### Reports

| Resource | Path | Common Operations |
|---|---|---|
| Sign-in logs | `/auditLogs/signIns` | GET list |
| Audit logs | `/auditLogs/directoryAudits` | GET list |
| Usage reports | `/reports/getOffice365ActiveUserDetail(period='D7')` | GET |

## OData Query Parameters

### $select — Choose fields

```
/users?$select=displayName,mail,userPrincipalName
```

### $filter — Filter results

```
/users?$filter=startsWith(displayName,'John')
/users?$filter=department eq 'Engineering'
/me/messages?$filter=isRead eq false
/users?$filter=accountEnabled eq true
```

### $top — Limit results

```
/users?$top=10
```

### $orderby — Sort results

```
/users?$orderby=displayName
/me/messages?$orderby=receivedDateTime desc
```

### $expand — Include related resources

```
/groups/{id}?$expand=members
/me/messages/{id}?$expand=attachments
```

### $count — Get count (requires ConsistencyLevel header)

```
/users/$count
```
Use with header: `ConsistencyLevel: eventual`

### $search — Search (requires ConsistencyLevel header)

```
/users?$search="displayName:John"
```
Use with header: `ConsistencyLevel: eventual`

### Combining parameters

```
/users?$select=displayName,mail&$filter=department eq 'Sales'&$top=25&$orderby=displayName
```

## Common Permission Scopes

| Scope | Description |
|---|---|
| `User.Read` | Read current user profile |
| `User.Read.All` | Read all user profiles |
| `User.ReadWrite.All` | Read and write all user profiles |
| `Mail.Read` | Read user mail |
| `Mail.ReadWrite` | Read and write user mail |
| `Mail.Send` | Send mail as user |
| `Calendars.Read` | Read user calendars |
| `Calendars.ReadWrite` | Read and write user calendars |
| `Group.Read.All` | Read all groups |
| `Group.ReadWrite.All` | Read and write all groups |
| `Directory.Read.All` | Read directory data |
| `Directory.ReadWrite.All` | Read and write directory data |
| `Files.Read` | Read user files |
| `Files.ReadWrite` | Read and write user files |
| `Sites.Read.All` | Read SharePoint sites |
| `Sites.ReadWrite.All` | Read and write SharePoint sites |
| `Team.ReadBasic.All` | Read basic team info |
| `Channel.ReadBasic.All` | Read basic channel info |
| `ChannelMessage.Read.All` | Read channel messages |
| `DeviceManagementManagedDevices.Read.All` | Read managed devices |
| `SecurityEvents.Read.All` | Read security events |
| `AuditLog.Read.All` | Read audit logs |

## Pagination

Graph API uses `@odata.nextLink` for pagination:

```json
{
  "@odata.nextLink": "https://graph.microsoft.com/beta/users?$skiptoken=...",
  "value": [...]
}
```

To get the next page, make a GET request to the `@odata.nextLink` URL directly.

## Batch Requests

Send multiple requests in a single HTTP call:

```
POST /beta/$batch
Content-Type: application/json

{
  "requests": [
    { "id": "1", "method": "GET", "url": "/me" },
    { "id": "2", "method": "GET", "url": "/me/messages?$top=5" }
  ]
}
```

## Error Response Format

```json
{
  "error": {
    "code": "Authorization_RequestDenied",
    "message": "Insufficient privileges to complete the operation.",
    "innerError": {
      "date": "2024-01-15T10:00:00",
      "request-id": "..."
    }
  }
}
```

## Tips

1. **Use $select** to reduce response size — only request fields you need
2. **Use $top** to limit results — avoid fetching thousands of records
3. **Check $filter support** — not all properties support filtering
4. **ConsistencyLevel header** is required for $count and $search on directory objects
5. **User identifier** can be the object ID or userPrincipalName (email)
6. **Group identifier** must be the object ID
7. **Date filters** use ISO 8601 format: `2024-01-15T00:00:00Z`
