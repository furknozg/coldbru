Requirements:

1. User Sends Message and budget Through a comm terminal
2. Agent proposes queries the customer dwh database to see if any reliable data can be used for said query
    options for querying:
        2.1 GraphQL with MCP and JS like DSL for constructing filters at a budget
        2.2 Agent directly interacts with a rest api, constructing the fields in a structured output
        2.3 (NOTE) the query should always determine outreach available (SELECT * FROM customer where email is not null or phone is not null)

3. Agent queries said service and returns the users allocated for the campaign
      3.1 This is simply the constructed query, being executed and returned to the user as a proposal

4. User Confirms or Rejects the allocation proposal:
    4.1 If the user confirms: continue to step 5
    4.2 If the user rejects: go back to step 2, mark proposal rejected

5. Agent proposes which platforms outreach can be done from: 
    5.1 SMS and Email mediums exist, however may not exist for all consumers due to GDPR regulations and consent
    5.2 Agent will determine the majority of the outreach available from the approved query, ask the user if some users don't have available outreach in said medium.
    5.3 User can either approve reduction of said users or can reach out to them via. means available
            5.3.1 If reduced, then only one proposal is needed
            5.3.2 If not reduced two proposals are needed.
  
6. Proposal of Notification:
    6.0: NOTE This should be indicated that it is for each unique language that this case is done for. 
    6.1 HTML is proposed for email if email is needed
    6.2 SMS is proposed if sms needed

7. Approval:
    7.1 if confirm continue to 8
    7.2 if reject go back to 5
 
8. Scheduling:
    8.1 user can ask right away
    8.2 user can use a datetime picker to indicate when it should start the campaign
        8.2.1 the job is scheduled to said time and the notification service applies the request from queue at said time

9. Notification Service: (asynchronous)
    9.1: Listens to notification queue
    9.2 Uses mock adapter for email and sms services as notification service interface adapter pattern

10. Bonus (click proxy & CPM analytics):
    10.1: shorturl click proxy terminal can be used to proxy from our servers to said campaign link, tracking the CPM metrics

11. Bonus (Analytics Worker & Service)



GOALS:
To achieve eventual consistency on notifications, potentially via. event driven design

To achieve minimal latency and status updates on interactions such as proposals, live websocket or eventstream might be useful in that regard

To achieve p95 latency in non llm endpoints around 200 ms (location invariant, locally could be less)

To achieve an infrastructure that is able to query the customer warehouse efficiently in terms of output tokens,
potential optimizations and nice-to-haves may or may not include:
  1. GraphQL Exposition of the customer service, giving the proposal api control over what it can obtain the schema, potentially opening up dynamic searching
  2. GraphQL bonus: construction of filters via. llm by a Domain spesific language that defines Javascript booleans, then compiled into graphql json
  3. Compiled sql queries: preferable over orm's in some regards (production ready migrations, indexing capabilities, Stored proc defns & LISTEN/NOTIFY and more)
  4. Inter service grpc
